#!/bin/bash

HOST="$1"

if [ -z "$HOST" ]; then
    echo "Usage: ./get-kubeconfig.sh <HOST>"
    exit 1
fi

ssh "kairos@$HOST" "sudo cat /etc/rancher/k3s/k3s.yaml" | sed "s/127\.0\.0\.1/$HOST/"
