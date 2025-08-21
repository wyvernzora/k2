#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   diff-manifests.sh [<repo-url>] [<extra dyff args>...]
#
# Defaults to the main repo on GitHub if no URL is provided.

REMOTE_URL="${1:-https://github.com/wyvernzora/k2.git}"
shift || true

# 1) Clone only the 'deploy' branch into a temp directory
TMPDIR="$(mktemp -d)"
git clone --branch deploy --depth 1 "$REMOTE_URL" "$TMPDIR" >/dev/null 2>&1

# 2) Build sorted lists of YAML manifests
LOCAL_LIST=$(mktemp)
REMOTE_LIST=$(mktemp)
ALL_LIST=$(mktemp)
trap 'rm -rf "$TMPDIR" "$LOCAL_LIST" "$REMOTE_LIST" "$ALL_LIST"' EXIT

find ./deploy -type f -name '*.k8s.yaml' \
  | sed 's|^./deploy/||' | sort > "$LOCAL_LIST"

find "$TMPDIR" -type f -name '*.k8s.yaml' \
  | sed "s#^${TMPDIR}/##" | sort > "$REMOTE_LIST"

# 3) Compute union of all paths
sort -u "$LOCAL_LIST" "$REMOTE_LIST" > "$ALL_LIST"

# 4) Build exclude arguments from .dyffignore
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

# 5) Loop over every manifest path
while IFS= read -r file; do
    LOCAL_PATH="./deploy/$file"
    REMOTE_PATH="$TMPDIR/$file"

    if [ ! -f "$REMOTE_PATH" ]; then
        echo "### ✨ \`$file\`"
        echo '```'
        cat "$LOCAL_PATH"
        echo '```'
        echo
        continue
    fi

    if [ ! -f "$LOCAL_PATH" ]; then
        echo "### 🗑️ \`$file\`"
        echo '```'
        cat "$REMOTE_PATH"
        echo '```'
        echo
        continue
    fi

    set +e
    diff_output=$(
        dyff between -ibs "${excludes[@]}" "$@" \
        "$REMOTE_PATH" \
        "$LOCAL_PATH"
    )
    rc=$?
    set -e
    # dyff returns 1 if differences were found, 0 if none, 2 on error
    if [ "$rc" -eq 1 ]; then
        echo "### ✏️ \`$file\`"
        echo -n '```'
        echo "$diff_output"
        echo '```'
        echo
    fi
done < "$ALL_LIST"

# 7) Cleanup
rm -rf "$TMPDIR" "$LOCAL_LIST" "$REMOTE_LIST" "$ALL_LIST"
