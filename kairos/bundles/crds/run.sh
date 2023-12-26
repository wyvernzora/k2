#!/bin/sh
set -ex

K3S_MANIFEST_DIR=${K3S_MANIFEST_DIR:-"/var/lib/rancher/k3s/server/manifests/"}
cp -r "./crds" "$K3S_MANIFEST_DIR"
