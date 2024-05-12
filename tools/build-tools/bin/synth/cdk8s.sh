#!/bin/sh
set -e

# Add helm repos if any are requested
jq -r '.helm.repos // [] | .[] | [.name, .url] | @tsv' package.json |
while IFS=$'\t' read -r name url; do
    helm repo add "$name" "$url"
done

ts-node app.ts

# Split CRDs out into a separate manifest for independent management
yq eval 'select(.kind == "CustomResourceDefinition")' dist/app.k8s.yaml > dist/crds.k8s.yaml
yq eval --inplace 'select(.kind != "CustomResourceDefinition")' dist/app.k8s.yaml

# Generate the CDK8s bindings for the CRDs
if [ ! -z "$(yq eval '.' dist/crds.k8s.yaml)" ]; then
    cdk8s import -l typescript -o dist dist/crds.k8s.yaml
fi
