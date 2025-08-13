#!/bin/sh
set -e

# Constants
APPS_DIR="apps"
DEPLOY_DIR="deploy"

synth_app() {
    local APP="$1"
    local APPROOT="$APPS_DIR/$APP"
    (
        cd "$APPROOT"
        mkdir -p "dist"
        echo "Synthesizing manifest for $APP"

        # Synth app CDK
        npx -y ts-node \
            -r tsconfig-paths/register \
            "deploy/index.ts"
        if [ ! -f "dist/app.k8s.yaml" ]; then
            echo "error: CDK synth did not produce expected output" 1>&2
            exit 1
        fi

        # Remove CRDs from resulting manifest
        yq eval --inplace 'select(.kind != "CustomResourceDefinition")' "dist/app.k8s.yaml"
    )
}

collect_manifests() {
    local APP="$1"
    local APP_SRC="$APPS_DIR/$APP"
    local APP_OUT="$DEPLOY_DIR/$APP"

    mkdir -p "$APP_OUT"
    cp "$APP_SRC/dist/app.k8s.yaml" "$APP_OUT/app.k8s.yaml"
    local CRD_MANIFEST_SRC="$APP_SRC/crds/crds.k8s.yaml"
    if [ -f "$CRD_MANIFEST_SRC" ]; then
        cp "$CRD_MANIFEST_SRC" "$APP_OUT/crds.k8s.yaml"
    fi
}

# Synthesize deployment manifests for each application
for app in "$APPS_DIR"/*/; do
    APP_NAME="$(basename $app)"
    synth_app "$APP_NAME"
    collect_manifests "$APP_NAME"
done

# Collect all ArgoCD manifests
# yq sorting is funky so making a detour through json/jq
yq eval-all -o=json "$APPS_DIR"/*/argocd.k8s.yaml |\
jq -s 'sort_by((.metadata.annotations["argocd.argoproj.io/sync-wave"] // "0" | tonumber), .metadata.name) | .[]' |\
yq -p=json > "$DEPLOY_DIR/app.k8s.yaml"
