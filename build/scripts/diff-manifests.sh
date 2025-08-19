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

# 2) build sorted file lists (YAML only)
find ./deploy -type f -name '*.k8s.yaml' \
  | sed 's|^./deploy/||' | sort > /tmp/local.txt

find "$TMPDIR" -type f -name '*.k8s.yaml' \
  | sed "s#^${TMPDIR}/##" | sort > /tmp/remote.txt

comm -23 /tmp/local.txt /tmp/remote.txt > /tmp/only-local.txt
comm -13 /tmp/local.txt /tmp/remote.txt > /tmp/only-remote.txt
comm -12 /tmp/local.txt /tmp/remote.txt > /tmp/common.txt

sed 's/^/+/' /tmp/only-local.txt
sed 's/^/+/' /tmp/only-remote.txt

# 4) for each file present in both, invoke dyff on that pair:
while IFS= read -r file; do
    set +e
    diff_output=$(dyff between -ibs "$TMPDIR/$file" "./deploy/$file")
    rc=$?
    set -e
    # dyff returns 1 if differences were found, 0 if none, 2 on error
    if [ "$rc" -eq 1 ]; then
        echo "Changes in $file:"
        printf '%s\n' "$diff_output"
    fi
done < /tmp/common.txt

# 5) cleanup
rm -rf "$TMPDIR" /tmp/{local,remote,only-local,only-remote,common}.txt
