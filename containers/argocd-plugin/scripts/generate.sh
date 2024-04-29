#!/bin/bash
set -e

if [ -f "./dist/manifest.k8s.yaml" ]; then
    cat "./dist/manifest.k8s.yaml"
else
    npx cdk8s-cli synth --stdout
fi
