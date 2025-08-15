#!/bin/bash
set -e

# Disable cdk8s update banner
cdk8s --version > $HOME/.cdk8s-cli.version

# Ensure that at least one argument is provided
if [ "$#" -lt 1 ]; then
  echo "Usage: $0 <app-path> [additional-helm-template-args]"
  exit 1
fi

# Define variables
APP_PATH="$1"
OUTPUT_DIR="$APP_PATH/crds"
shift

# Import CRDs using cdk8s
echo "Generating CRD constructs for $(basename $APP_PATH)"
cdk8s import -l typescript -o "$OUTPUT_DIR" "$APP_PATH/crds/crds.k8s.yaml" > /dev/null
