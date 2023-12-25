#!/bin/bash

HOST="$1"

if [ -z "$HOST" ]; then
    echo "Usage: ./get-node-token.sh <HOST>"
    exit 1
fi

ssh "kairos@$HOST" "sudo cat /var/lib/rancher/k3s/server/token"
