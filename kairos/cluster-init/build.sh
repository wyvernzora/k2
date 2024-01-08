#!/bin/sh
# Builds the cluster-init manifests.
# Note that this script in run from Docker build context.
set -ex

OUTPUT=/manifests/k2-cluster-init.yaml

# Install required build tools
apk add --no-cache moreutils helm yq

# Set up helm repos
helm repo add argo https://argoproj.github.io/argo-helm
helm repo add 1password https://1password.github.io/connect-helm-charts
helm repo update

# Create output directory
mkdir -p /manifests

# Prepare Argo CD values file
yq -i '.argo-cd' argocd/values.yaml

# Render cluster-init manifest
{
  # Core Namespace
  cat manifests/namespace.yaml

  # Argo CD
  helm template \
      --namespace k2-core \
      --release-name argocd \
      --values argocd/values.yaml \
      argo/argo-cd
  cat manifests/root-app.yaml

  # 1Password Connect
  cat manifests/op-secret.yaml
  helm template \
      --namespace k2-core \
      --release-name 1password-connect \
      --set operator.create=true \
      1password/connect

} >> "$OUTPUT"
