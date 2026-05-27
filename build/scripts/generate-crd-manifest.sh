#!/bin/bash
set -e

# Ensure that at least one argument is provided
if [ "$#" -lt 1 ]; then
  echo "Usage: $0 <app-path> [additional-helm-template-args]"
  exit 1
fi

# Define variables
APP_PATH="$1"  # First argument should be the path to an app
APP_NAME="$(basename "$APP_PATH")"
OUTPUT_DIR="$APP_PATH/crds"
OUTPUT_FILE="crds.k8s.yaml"
CRDS_BACKUP=""

# Shift removes the first argument so that "$@" now contains only additional arguments for helm template
shift

# Extract index-backed repositories from Chart.yaml using yq. OCI
# dependencies are resolved directly by `helm dependency build` and cannot be
# added with `helm repo add`.
REPOS=$(yq eval '.dependencies[]?.repository // ""' "$APP_PATH/Chart.yaml" | sort | uniq)
HAS_HELM_REPOS=false

# Add all necessary repositories
for repo in $REPOS; do
  if [ -z "$repo" ] || [[ "$repo" == oci://* ]]; then
    continue
  fi
  # Helm will not throw an error if the repository is already added
  helm repo add $(echo $repo | awk -F'/' '{print $NF}') $repo
  HAS_HELM_REPOS=true
done

# Update repository cache
if [ "$HAS_HELM_REPOS" = true ]; then
  helm repo update
fi

restore_crds() {
  if [ -n "$CRDS_BACKUP" ] && [ -d "$CRDS_BACKUP/crds" ]; then
    rm -rf "$OUTPUT_DIR"
    mv "$CRDS_BACKUP/crds" "$OUTPUT_DIR"
    rmdir "$CRDS_BACKUP"
    CRDS_BACKUP=""
  fi
}

trap restore_crds EXIT

# Build the dependencies
echo "Building Helm chart dependencies..."
helm dependency build "$APP_PATH"

# K2 stores committed upstream CRDs under apps/<name>/crds/, which Helm also
# treats as root-chart CRDs. Move them aside while rendering so extraction only
# sees CRDs emitted by current chart dependencies.
if [ -d "$OUTPUT_DIR" ]; then
  CRDS_BACKUP="$(mktemp -d)"
  mv "$OUTPUT_DIR" "$CRDS_BACKUP/crds"
fi

# Use Helm to template the complete chart, including dependencies, passing additional args
tmplt_output=$(mktemp)  # Create a temporary file to capture output
helm template "$APP_NAME" "$APP_PATH" --include-crds "$@" > "$tmplt_output"

# Restore the committed CRD directory before replacing the generated manifest.
restore_crds
mkdir -p "$OUTPUT_DIR"

# Extract CRDs using yq and save to the output file
echo "Extracting CRDs..."
yq eval '. | select(.kind == "CustomResourceDefinition")' "$tmplt_output" > "$OUTPUT_DIR/$OUTPUT_FILE"

# Clean up temporary files
rm "$tmplt_output"

echo "CRDs have been extracted to $OUTPUT_DIR/$OUTPUT_FILE"
