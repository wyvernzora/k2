#!/bin/bash
set -e
set -x

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
cdk8s -l typescript -o "$OUTPUT_DIR" "$APP_PATH/crds/crds.k8s.yaml"
