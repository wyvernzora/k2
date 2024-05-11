#!/bin/sh
set -e

command_exists()
{
    command -v "$1" &> /dev/null
}

print_error()
{
    echo "$1" 1>&2
}

KUSTOMIZE_CMD=""
if command_exists "kustomize"; then
    KUSTOMIZE_CMD="kustomize build"
elif command_exists "kubectl"; then
    KUSTOMIZE_CMD="kubectl kustomize"
fi

if [ -z "$KUSTOMIZE_CMD" ]; then
    print_error "Could not determine kustomize command"
    print_error "Make sure kustomize or kubectl are available on PATH"
    exit 1
fi

mkdir -p "./dist"
$KUSTOMIZE_CMD "." > "./dist/manifest.k8s.yaml"
