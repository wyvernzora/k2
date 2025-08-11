#!/bin/sh
set -e

# Constants
APPS_DIR="packages"
DEPLOY_DIR="deploy"

# Synth package
synth_app() {
    local APP="$1"
    local APPROOT="$APPS_DIR/$APP"
    (
        cd "$APPROOT"
        mkdir -p "dist"
        echo "Synthesizing manifest for $APP"

        # Synth app CDK
        npx -y ts-node "app.ts"
        if [ ! -f "dist/app.k8s.yaml" ]; then
            echo "error: CDK synth did not produce expected output" 1>&2
            exit 1
        fi

        # Remove CRDs from resulting manifest
        yq eval --inplace 'select(.kind != "CustomResourceDefinition")' "dist/app.k8s.yaml"
    )
}

# Install NPM dependencies
npm ci

# Synthesize deployment manifests for each application
for app in "$APPS_DIR"/*/; do
    synth_app "$(basename $app)"
done

# Run release process
npx -y '@k2/release'
