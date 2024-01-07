#!/bin/sh
# Builds the cluster-init manifests.
# Note that this script in run from Docker build context.
set -ex

# Install required build tools
apk add --no-cache moreutils helm yq

# Set up helm repos
helm repo add argo https://argoproj.github.io/argo-helm
helm repo add 1password https://1password.github.io/connect-helm-charts
helm repo update

# Create output directory
mkdir -p /manifests

# Render ArgoCD manifests
yq -i '.argo-cd' argocd/values.yaml
cat argocd/namespace.yaml > /manifests/argocd.yaml
helm template \
    --namespace argocd \
    --create-namespace \
    --release-name argocd \
    --values argocd/values.yaml \
    argo/argo-cd >> /manifests/argocd.yaml
cat argocd/root-app.yaml >> /manifests/argocd.yaml

# Render 1Password Operator manifests
cat 1password/namespace.yaml > /manifests/1password.yaml
cat 1password/secret.yaml >> /manifests/1password.yaml
helm template \
    --namespace 1password \
    --create-namespace \
    --release-name 1password-connect \
    --set operator.create=true \
    1password/connect >> /manifests/1password.yaml
