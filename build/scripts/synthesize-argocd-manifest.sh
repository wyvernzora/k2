#!/bin/bash
set -e

# Constants
APPS_DIR="apps"
DEPLOY_DIR="deploy"

mkdir -p "$DEPLOY_DIR"

# Collect all ArgoCD manifests
# yq sorting is funky so making a detour through json/jq
yq eval-all -o=json "$APPS_DIR"/*/argocd.k8s.yaml |\
jq -s 'sort_by((.metadata.annotations["argocd.argoproj.io/sync-wave"] // "0" | tonumber), .metadata.name) | .[]' |\
yq -p=json > "$DEPLOY_DIR/app.k8s.yaml"
