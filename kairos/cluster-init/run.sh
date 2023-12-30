#!/bin/sh
set -ex

K3S_MANIFEST_DIR=${K3S_MANIFEST_DIR:-/var/lib/rancher/k3s/server/manifests}
mkdir -p "$K3S_MANIFEST_DIR"

kairos-agent render-template -f ./manifests/argocd.yaml > "$K3S_MANIFEST_DIR/argocd.yaml"
kairos-agent render-template -f ./manifests/1password.yaml > "$K3S_MANIFEST_DIR/1password.yaml"
