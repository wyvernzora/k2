#!/bin/bash

ROOT=$(dirname $0)
TYPE="$1"
HOST="$2"

help() {
    echo -e "Usage: ./configure-node.sh <type> <host>\n"
    echo -e "Available node types:\n"
    echo -e "\tbootstrap\t\tFirst master node in the cluster"
    echo -e "\tmaster\t\tSubsequent master nodes"
    echo -e "\tworker\t\tWorker nodes"
}


CONFIG_FILE=""
case "$TYPE" in
    b|bootstrap)
        CONFIG_FILE="$ROOT/../cloud-config/bootstrap.yaml"
        ;;
    m|master)
        CONFIG_FILE="$ROOT/../cloud-config/master.yaml"
        ;;
    w|worker)
        CONFIG_FILE="$ROOT/../cloud-config/worker.yaml"
        ;;
    *)
        help
        echo -e "\nUnknown node type: $TYPE"
        exit 1
        ;;
esac

echo -e "Using config file $CONFIG_FILE"
echo -e "When asked for password, use 'kairos'"
op inject -i "$CONFIG_FILE" | ssh kairos@$HOST "cat > cloud-config.yaml && sudo kairos-agent manual-install --reboot cloud-config.yaml"
