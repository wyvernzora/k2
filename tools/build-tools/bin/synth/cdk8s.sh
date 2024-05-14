#!/bin/sh
set -e

ENTRYPOINT="${1:-"app.ts"}"

ts-node "$ENTRYPOINT"
if [ ! -f dist/app.k8s.yaml ]; then
    echo "error: CDK synth did not produce expected output" 1>&2
    exit 1
fi

# Split CRDs out into a separate manifest for independent management
yq eval 'select(.kind == "CustomResourceDefinition")' dist/app.k8s.yaml > dist/crds.k8s.yaml
yq eval --inplace 'select(.kind != "CustomResourceDefinition")' dist/app.k8s.yaml

# Generate the CDK8s bindings for the CRDs
if [ ! -z "$(yq eval '.' dist/crds.k8s.yaml)" ]; then
    cdk8s import -l typescript -o dist dist/crds.k8s.yaml
fi
