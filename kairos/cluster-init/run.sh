#!/bin/sh
set -ex

K3S_MANIFEST_DIR=${K3S_MANIFEST_DIR:-/var/lib/rancher/k3s/server/manifests}
mkdir -p "$K3S_MANIFEST_DIR"

kairos-agent render-template -f ./manifests/k2-cluster-init.yaml > "$K3S_MANIFEST_DIR/k2-cluster-init.yaml"
