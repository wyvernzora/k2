#!/bin/bash
set -e

# Log some information for diagnostics
pwd 1>&2

# Install npm dependencies
npm ci --include-workspace-root 1>&2

# Run nx build on the package in question
npx -y nx run-many -t build --output-style=static 1>&2
