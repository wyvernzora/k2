#!/bin/sh
set -e

# Add helm repos if any are requested
jq -r '.helm.repos // [] | .[] | [.name, .url] | @tsv' package.json |
while IFS=$'\t' read -r name url; do
    helm repo add "$name" "$url"
done

ts-node app.ts
