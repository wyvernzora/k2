#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <changed-files-path> <force-all>" >&2
  exit 2
fi

changed_files_path=$1
force_all=$2
repo_root=$(git rev-parse --show-toplevel)

matches_pattern() {
  local file=$1
  local pattern=$2

  [[ "$file" == $pattern ]]
}

image_changed() {
  local metadata_path=$1

  if [[ "$force_all" == "true" ]]; then
    return 0
  fi

  while IFS= read -r pattern; do
    [[ -n "$pattern" ]] || continue

    while IFS= read -r changed_file; do
      [[ -n "$changed_file" ]] || continue

      if matches_pattern "$changed_file" "$pattern"; then
        return 0
      fi
    done < "$changed_files_path"
  done < <(jq -r '.watch[]?' "$metadata_path")

  return 1
}

images=()
shopt -s nullglob
for metadata_path in "$repo_root"/images/*/image.json; do
  if image_changed "$metadata_path"; then
    images+=("$metadata_path")
  fi
done

if [[ ${#images[@]} -eq 0 ]]; then
  echo "[]"
  exit 0
fi

jq -s -c '
  map({
    name: .name,
    image: .image,
    packageName: .packageName,
    earthlyTarget: .earthlyTarget,
    versionTag: (.versionTag // ""),
    latestOnMain: (.latestOnMain // true),
    keepVersions: (.retention.keepVersions // 5)
  })
' "${images[@]}"
