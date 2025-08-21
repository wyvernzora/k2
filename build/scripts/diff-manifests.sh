#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   diff-manifests.sh [<repo-url>] [<extra dyff args>...]
#
# Defaults to the main repo on GitHub if no URL is provided.

REMOTE_URL="${1:-https://github.com/wyvernzora/k2.git}"
shift || true

if [ ! -d "deploy" ]; then
    echo "🚨 Local build artifacts not found; did you run +k8s-manifests?" 1>&2
    exit 2
fi

# 1) Clone only the 'deploy' branch into a temp directory
TMPDIR="$(mktemp -d)"
git clone --branch deploy --depth 1 "$REMOTE_URL" "$TMPDIR" >/dev/null 2>&1
rm -rf "$TMPDIR/.git"

# 2) Build exclude arguments from .dyffignore
excludes=()
DYFFIGNORE=".dyffignore"
if [[ -f "$DYFFIGNORE" ]]; then
    while IFS= read -r ptr; do
    # skip empty lines & comments
    [[ -z "$ptr" || "$ptr" =~ ^# ]] && continue

    # ensure it starts with a slash (JSON Pointer)
    [[ "$ptr" != /* ]] && ptr="/$ptr"

    excludes+=( "--exclude" "$ptr" )
    done < "$DYFFIGNORE"
fi

# 3) Detect add/delete/modify/rename between remote ($TMPDIR) and local (deploy/)
#    using git diff --no-index -M --name-status
set +e
mapfile -t diffs < <(
  git diff --no-index --name-status -M --color=never "$TMPDIR" "deploy"
)
set -e

function print-diff() {
    local HEADING="$1"
    local DIFF="$2"

    # —————— A) Strip *all* leading newlines from $DIFF ——————
    #   ${DIFF%%[!$'\n']*} expands to the longest prefix of only newlines.
    #   Then we remove that from the front.
    DIFF="${DIFF#"${DIFF%%[!$'\n']*}"}"

    echo "$HEADING"
    printf '```\n%s\n```\n\n' "$DIFF"
    echo
}

function cleanup-path() {
    local path="$1"
    if [[ "$path" == "$TMPDIR/"* ]]; then
      path="${path#$TMPDIR/}"
    fi
    if [[ "$path" == "deploy/"* ]]; then
      path="${path#deploy/}"
    fi
    echo "$path"
}

for entry in "${diffs[@]}"; do
  # Fields are tab-separated: status [old_path] [new_path]
  IFS=$'\t' read -r status path1 path2 <<< "$entry"
  path1="$(cleanup-path "$path1")"
  path2="$(cleanup-path "$path2")"

  echo "$status $path1 $path2" 1>&2

  case "$status" in
    A)
      # Added locally
      print-diff "### ✨ \`$path1\`" "$(cat "deploy/$path1")"
      ;;
    D)
      # Deleted locally ( only in remote )
      print-diff "### 🗑️ \`$path1\`" "$(cat "$TMPDIR/$path1")"
      ;;
    M)
      # Modified
      set +e
      diff_output=$(
        dyff between -ibs "${excludes[@]}" "$@" \
          "$TMPDIR/$path1" "deploy/$path1"
      )
      rc=$?
      set -e
      if [ "$rc" -eq 1 ]; then
        print-diff "### ✏️ \`$path1\`" "${diff_output#$'\n'}"
      fi
      ;;
    R*)
      # Renamed or moved
      set +e
      diff_output=$(
        dyff between -ibs "${excludes[@]}" "$@" \
          "$TMPDIR/$path1" "deploy/$path2"
      )
      rc=$?
      set -e

      if [ "$rc" -eq 1 ]; then
        print-diff "### 🔀 \`$path1\` → \`$path2\`" "${diff_output#$'\n'}"
      else
        print-diff "### 🔀 \`$path1\` → \`$path2\`" "no changes"
      fi
      ;;
    *)
      # ignore other statuses (e.g. C, T)
      ;;
  esac
done

# 4) Cleanup
rm -rf "$TMPDIR"
