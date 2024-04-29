#!/bin/sh
set -e

mkdir -p dist

# Generate the full CRD manifest via Kustomize
if command -v kustomize &> /dev/null; then
    kustomize build . > dist/manifest.k8s.yaml
elif command -v kubectl &> /dev/null; then
    kubectl kustomize . > dist/manifest.k8s.yaml
else
    echo "Cannot find suitable kustomize command"
    exit 1
fi

# Generate cdk8s constructs for the CRDs
cdk8s import -l typescript -o dist dist/manifest.k8s.yaml

# Generate the main export file
echo "// Generated file, do not edit" > dist/index.d.ts
for f in dist/*.ts; do
    echo "export * from \"./$(basename "$f")\";" >> dist/index.d.ts
done
