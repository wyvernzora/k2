#!/bin/sh
set -ex

K3S_MANIFEST_DIR=${K3S_MANIFEST_DIR:-/var/lib/rancher/k3s/server/manifests}
mkdir -p "$K3S_MANIFEST_DIR"
cp ./manifests/k2-bootstrap.yaml "$K3S_MANIFEST_DIR"
