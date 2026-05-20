#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: resolve-ci-targets.sh <plans.json> <changed-files.txt> <force-all>

Emits a GitHub Actions matrix include array for enabled Kairos image targets
whose common inputs or selected overlays changed.
EOF
}

if [[ $# -ne 3 ]]; then
  usage
  exit 64
fi

plans_path="$1"
changed_files_path="$2"
force_all="$3"

case "${force_all}" in
  true | false) ;;
  *)
    echo "force-all must be true or false, got ${force_all}" >&2
    exit 64
    ;;
esac

mapfile -t changed_files <"${changed_files_path}"

common_input_changed() {
  local path="$1"

  case "${path}" in
    .github/workflows/kairos-image-build.yaml | \
    kairos/Earthfile | \
    kairos/targets.yaml | \
    kairos/versions.env | \
    kairos/image-build/Dockerfile | \
    kairos/image-build/go.mod | \
    kairos/image-build/go.sum)
      return 0
      ;;
    kairos/image-build/cmd/* | \
    kairos/image-build/internal/* | \
    kairos/image-build/scripts/* | \
    kairos/node-init/*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

overlay_input_changed() {
  local path="$1"
  local overlay

  [[ "${path}" == kairos/image-build/overlays/* ]] || return 1
  local rel="${path#kairos/image-build/overlays/}"

  for overlay in "${target_overlays[@]}"; do
    if [[ "${rel}" == "${overlay}" || "${rel}" == "${overlay}/"* ]]; then
      return 0
    fi
  done

  return 1
}

target_changed() {
  local path

  if [[ "${force_all}" == "true" ]]; then
    return 0
  fi

  for path in "${changed_files[@]}"; do
    [[ -n "${path}" ]] || continue
    if common_input_changed "${path}" || overlay_input_changed "${path}"; then
      return 0
    fi
  done

  return 1
}

runner_for_arch() {
  local arch="$1"

  case "${arch}" in
    amd64) echo "ubuntu-24.04" ;;
    arm64) echo "ubuntu-24.04-arm" ;;
    *)
      echo "unsupported target arch ${arch}" >&2
      exit 1
      ;;
  esac
}

matrix_file="$(mktemp)"
trap 'rm -f "${matrix_file}"' EXIT
: >"${matrix_file}"

while IFS= read -r target_json; do
  target="$(jq -r '.target' <<<"${target_json}")"
  arch="$(jq -r '.arch' <<<"${target_json}")"
  mapfile -t target_overlays < <(jq -r '.overlays[]?' <<<"${target_json}")

  if ! target_changed; then
    echo "Skipping ${target}: no target-specific inputs changed" >&2
    continue
  fi

  jq -cn \
    --arg target "${target}" \
    --arg arch "${arch}" \
    --arg runner "$(runner_for_arch "${arch}")" \
    '{target: $target, arch: $arch, runner: $runner}' >>"${matrix_file}"
done < <(jq -c '.targets[] | select(.enabled == true)' "${plans_path}")

jq -cs '.' "${matrix_file}"
