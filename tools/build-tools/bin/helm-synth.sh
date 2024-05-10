#!/bin/sh
set -e

mkdir -p dist
helm dependency build
helm template --namespace "$1" --release-name "$2"  . > "dist/manifest.k8s.yaml"
