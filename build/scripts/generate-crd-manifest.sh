#!/bin/bash
set -e
set -x

# Ensure that at least one argument is provided
if [ "$#" -lt 1 ]; then
  echo "Usage: $0 <app-path> [additional-helm-template-args]"
  exit 1
fi

# Define variables
APP_PATH="$1"  # First argument should be the path to an app
OUTPUT_DIR="crds"
OUTPUT_FILE="crds.k8s.yaml"

# Shift removes the first argument so that "$@" now contains only additional arguments for helm template
shift

# Extract repositories and names from Chart.yaml using yq
REPOS=$(yq eval '.dependencies | .[] | .repository' "$APP_PATH/Chart.yaml" | sort | uniq)

# Add all necessary repositories
for repo in $REPOS; do
  # Helm will not throw an error if the repository is already added
  helm repo add $(echo $repo | awk -F'/' '{print $NF}') $repo
done

# Update repository cache
helm repo update

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Build the dependencies
echo "Building Helm chart dependencies..."
helm dependency build "$APP_PATH"

# Use Helm to template the complete chart, including dependencies, passing additional args
tmplt_output=$(mktemp)  # Create a temporary file to capture output
helm template "$APP_PATH" "$@" > "$tmplt_output"

# Extract CRDs using yq and save to the output file
echo "Extracting CRDs..."
yq eval '. | select(.kind == "CustomResourceDefinition")' "$tmplt_output" > "$OUTPUT_DIR/$OUTPUT_FILE"

# Clean up temporary files
rm "$tmplt_output"

echo "CRDs have been extracted to $OUTPUT_DIR/$OUTPUT_FILE"
