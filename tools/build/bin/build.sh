#!/bin/sh
set -e

# Synth the specified CDK8s app entry file.
synth_app() {
    ts-node "$1"
    if [ ! -f dist/app.k8s.yaml ]; then
        echo "error: CDK synth did not produce expected output" 1>&2
        exit 1
    fi

    # Split CRDs out into a separate manifest for independent management
    yq eval 'select(.kind == "CustomResourceDefinition")' dist/app.k8s.yaml > dist/crds.k8s.yaml
    yq eval --inplace 'select(.kind != "CustomResourceDefinition")' dist/app.k8s.yaml

    # If CRD file is empty, remove it; otherwise codegen CDK8s constructs
    if [ -z "$(yq eval '.' dist/crds.k8s.yaml)" ]; then
        rm dist/crds.k8s.yaml
    else
        cdk8s import -l typescript -o dist dist/crds.k8s.yaml
    fi
}

# Since Helm charts do not have a standard way of handling CRDs, really the best way
# to bootstrap a package with its CRD constructs is to actually synth the chart and
# pluck CRDs out of the results. Which is why he have the special bootstrap.ts file
# to provide the minimal synth-able chart app to extract CRDs from.
if [ -f bootstrap.ts ]; then
    synth_app bootstrap.ts
fi

# Now synth the app itself, given that at this point we have the CRD constructs generated
# and ready for import.
synth_app app.ts
