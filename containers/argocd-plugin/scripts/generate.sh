#!/bin/bash
set -e

MANIFEST=""
if [ -f "package.json" ]; then
    PKG_NAME="$(jq -cr '.name' "package.json")"
    echo "Package: $PKG_NAME" 1>&2
    MANIFEST="$(npx require-resolve-cli "${PKG_NAME}/manifest")"
    echo "Manifest: $MANIFEST" 1>&2
fi

if [ -f "$MANIFEST" ]; then
    cat "$MANIFEST"
else
    npx cdk8s-cli synth --stdout
fi
