#!/bin/bash
set -e
set -x

# Build manifests
npm ci
node -r tsconfig-paths/register -r ts-node/register app.ts

# Re-organize manifests into deploy directory
for file in ./dist/*.k8s.yaml; do
  app_name=$(basename "$file" .k8s.yaml) # Get the app name by removing the extension
  mkdir -p "deploy/$app_name"            # Create the subdirectory for the app
  mv "$file" "deploy/$app_name/app.k8s.yaml"  # Rename and move the file
done
