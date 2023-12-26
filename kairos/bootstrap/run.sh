#!/bin/bash
set -ex

K3S_MANIFEST_DIR=${K3S_MANIFEST_DIR:-/var/lib/rancher/k3s/server/manifests/}
TEMP_DIR=$(mktemp -d)

# Copy CRDs to auto-deploy directory
cp -r "./crds" "$K3S_MANIFEST_DIR"

# Render templates into a temporary directory
for FILE in templates/*; do 
    OUTFILE="$TEMP_DIR/${FILE##*/}"
    echo "Rendering $FILE to $OUTFILE"
    kairos-agent render-template -f "$FILE" > "$OUTFILE"
done;

# Copy rendered manifests to output location
echo "Copying rendered files to k3s manifest directory"
mkdir -p "$K3S_MANIFEST_DIR"
cp "$TEMP_DIR"/* "$K3S_MANIFEST_DIR"
