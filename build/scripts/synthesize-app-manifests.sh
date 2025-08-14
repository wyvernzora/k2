#!/bin/bash
set -e

# Constants
APP_ROOT="$1"
APP_NAME="$(basename "$APP_ROOT")"
APP_OUT="deploy/$APP_NAME"

# Switch CWD to app root and synthesize the manifests
echo "Synthesizing manifest for $APP_NAME"
(
    cd "$APP_ROOT"
    mkdir -p "dist"

    npx -y ts-node \
        -r tsconfig-paths/register \
        "deploy/index.ts"
    if [ ! -f "dist/app.k8s.yaml" ]; then
        echo "error: CDK synth did not produce expected output" 1>&2
        exit 1
    fi

    # Remove CRDs from resulting manifest
    # CRDs are managed manually via crds/crds.k8s.yaml
    yq eval --inplace 'select(.kind != "CustomResourceDefinition")' "dist/app.k8s.yaml"


)

# Move app manifest to deploy directory
mkdir -p "$APP_OUT"
cp "$APP_ROOT/dist/app.k8s.yaml" "$APP_OUT/app.k8s.yaml"

# Move CRD manifest to deploy directory (if present)
if [ -f "$APP_ROOT/crds/crds.k8s.yaml" ]; then
    cp "$APP_ROOT/crds/crds.k8s.yaml" "$APP_OUT/crds.k8s.yaml"
fi
