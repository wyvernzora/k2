#!/bin/sh
set -e

# Add helm repos if any are requested
jq -r '.helm.repos // [] | .[] | [.name, .url] | @tsv' package.json |
while IFS=$'\t' read -r name url; do
    helm repo add "$name" "$url"
done

mkdir -p dist
helm dependency build > /dev/null
helm template --namespace "$1" --release-name "$2"  . > "dist/manifest.k8s.yaml"
