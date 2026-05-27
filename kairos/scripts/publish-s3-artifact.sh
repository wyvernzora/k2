#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  publish-s3-artifact.sh <target> <artifact-dir>

Environment:
  K2_KAIROS_IMAGE_BUCKET          Required S3 bucket name.
  K2_KAIROS_IMAGE_PREFIX          Optional S3 key prefix.
  K2_KAIROS_IMAGE_KEEP            Number of builds to keep per target. Default: 3.
  K2_KAIROS_PUBLISH_MANIFEST      Optional path for the generated publish manifest.
  K2_KAIROS_IMAGE_STORAGE_CLASS   Storage class for image blobs. Default: ONEZONE_IA.
EOF
}

die() {
  printf 'publish-s3-artifact: %s\n' "$*" >&2
  exit 1
}

log() {
  printf 'publish-s3-artifact: %s\n' "$*" >&2
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

s3_key() {
  local prefix="$1"
  local suffix="$2"

  prefix="${prefix#/}"
  prefix="${prefix%/}"
  suffix="${suffix#/}"
  if [[ -n "${prefix}" ]]; then
    printf '%s/%s\n' "${prefix}" "${suffix}"
  else
    printf '%s\n' "${suffix}"
  fi
}

copy_if_present() {
  local source="$1"
  local destination="$2"
  local storage_class="$3"

  [[ -f "${source}" ]] || return 0
  log "uploading ${source} to s3://${bucket}/${destination}"
  aws s3 cp "${source}" "s3://${bucket}/${destination}" --storage-class "${storage_class}"
}

prune_old_builds() {
  local target="$1"
  local keep="$2"
  local prefix="$3"
  local list_file

  [[ "${keep}" =~ ^[0-9]+$ ]] || die "K2_KAIROS_IMAGE_KEEP must be a number"
  if ((keep < 1)); then
    die "K2_KAIROS_IMAGE_KEEP must be at least 1"
  fi

  list_file="$(mktemp)"
  aws s3api list-objects-v2 \
    --bucket "${bucket}" \
    --prefix "$(s3_key "${prefix}" "images/${target}/")" \
    --output json >"${list_file}"

  stale_prefixes=()
  while IFS= read -r stale_prefix; do
    [[ -n "${stale_prefix}" ]] || continue
    stale_prefixes+=("${stale_prefix}")
  done < <(
    jq -r --argjson keep "${keep}" '
      [.Contents[]?
        | select(.Key | endswith("/manifest.json"))
        | {key: .Key, modified: .LastModified}]
      | sort_by(.modified)
      | reverse
      | .[$keep:][]
      | .key
      | sub("/manifest.json$"; "/")
    ' "${list_file}"
  )
  rm -f "${list_file}"

  for stale_prefix in "${stale_prefixes[@]}"; do
    log "pruning s3://${bucket}/${stale_prefix}"
    aws s3 rm "s3://${bucket}/${stale_prefix}" --recursive
  done
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

[[ $# -eq 2 ]] || {
  usage >&2
  exit 1
}

need aws
need jq

target="$1"
artifact_dir="$2"
bucket="${K2_KAIROS_IMAGE_BUCKET:-}"
prefix="${K2_KAIROS_IMAGE_PREFIX:-}"
keep="${K2_KAIROS_IMAGE_KEEP:-3}"
image_storage_class="${K2_KAIROS_IMAGE_STORAGE_CLASS:-ONEZONE_IA}"
git_sha="${GITHUB_SHA:-}"
git_ref="${GITHUB_REF_NAME:-${GITHUB_REF:-}}"
run_id="${GITHUB_RUN_ID:-}"
run_attempt="${GITHUB_RUN_ATTEMPT:-}"
run_url=""

[[ -n "${bucket}" ]] || die "K2_KAIROS_IMAGE_BUCKET is required"
[[ -n "${git_sha}" ]] || die "GITHUB_SHA is required"
[[ -d "${artifact_dir}" ]] || die "artifact directory not found: ${artifact_dir}"
[[ -f "${artifact_dir}/artifact-manifest.json" ]] || die "missing ${artifact_dir}/artifact-manifest.json"

if [[ -n "${GITHUB_SERVER_URL:-}" && -n "${GITHUB_REPOSITORY:-}" && -n "${run_id}" ]]; then
  run_url="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${run_id}"
fi

image_prefix="$(s3_key "${prefix}" "images/${target}/${git_sha}")"
latest_prefix="$(s3_key "${prefix}" "latest/${target}")"
publish_manifest="${K2_KAIROS_PUBLISH_MANIFEST:-}"
if [[ -z "${publish_manifest}" ]]; then
  publish_manifest="$(mktemp)"
else
  mkdir -p "$(dirname "${publish_manifest}")"
fi

jq \
  --arg target "${target}" \
  --arg gitSha "${git_sha}" \
  --arg gitRef "${git_ref}" \
  --arg runId "${run_id}" \
  --arg runAttempt "${run_attempt}" \
  --arg runUrl "${run_url}" \
  --arg bucket "${bucket}" \
  --arg imagePrefix "${image_prefix}" \
  '{
    target: $target,
    git: {
      sha: $gitSha,
      ref: $gitRef
    },
    githubActions: {
      runId: $runId,
      runAttempt: $runAttempt,
      runUrl: $runUrl
    },
    s3: {
      bucket: $bucket,
      prefix: $imagePrefix
    },
    artifact: .
  }' \
  "${artifact_dir}/artifact-manifest.json" >"${publish_manifest}"

shopt -s nullglob
image_files=("${artifact_dir}"/*.raw.xz "${artifact_dir}"/*.iso)
shopt -u nullglob
(( ${#image_files[@]} > 0 )) || die "no bootable image files found under ${artifact_dir}"

for image_file in "${image_files[@]}"; do
  copy_if_present "${image_file}" "${image_prefix}/$(basename "${image_file}")" "${image_storage_class}"
done

copy_if_present "${artifact_dir}/SHA256SUMS" "${image_prefix}/SHA256SUMS" STANDARD
copy_if_present "${artifact_dir}/artifact-manifest.json" "${image_prefix}/artifact-manifest.json" STANDARD
copy_if_present "${publish_manifest}" "${image_prefix}/manifest.json" STANDARD
copy_if_present "${publish_manifest}" "${latest_prefix}/manifest.json" STANDARD

prune_old_builds "${target}" "${keep}" "${prefix}"
