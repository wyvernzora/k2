#!/bin/sh
set -e

mkdir -p dist

# Generate the full CRD manifest via Kustomize
k2-kustomize . > dist/manifest.k8s.yaml

# Generate cdk8s constructs for the CRDs
cdk8s import -l typescript -o dist dist/manifest.k8s.yaml

# Generate the main export file
echo "// Generated file, do not edit" > dist/index.d.ts
for f in dist/*.ts; do
    echo "export * from \"./$(basename "$f")\";" >> dist/index.d.ts
done
