#!/usr/bin/env bash
set -euo pipefail

commands=(
  awk
  bash
  base64
  curl
  diff
  dig
  dyff
  envsubst
  find
  grep
  gzip
  jq
  kubectl
  nc
  nslookup
  openssl
  sed
  sha256sum
  tar
  unzip
  wget
  yq
)

for command in "${commands[@]}"; do
  command -v "${command}" >/dev/null
done

bash --version >/dev/null
curl --version >/dev/null
dyff version >/dev/null
jq --version >/dev/null
kubectl version --client=true >/dev/null
openssl version >/dev/null
yq --version >/dev/null
