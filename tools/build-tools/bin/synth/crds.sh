#!/bin/sh
set -e

mkdir -p dist

# Generate the full CRD manifest via Kustomize
k2-synth-kustomize . > dist/manifest.k8s.yaml

# Generate cdk8s constructs for the CRDs
cdk8s import -l typescript -o dist dist/manifest.k8s.yaml
