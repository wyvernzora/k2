#!/bin/sh

function print_available_playbooks() {
    echo -e "Available playbooks:\n"
    for FILE in playbooks/*.yml; do
        echo -e "  $(basename "${FILE%.yml}")\t\t\t$(head -n 1 "$FILE" | sed "s/^# //")" 
    done
}

function help() {
    echo -e "Usage: docker run [options] ghcr.io/wyvernzora/k2-ansible <playbook-name>\n"
    print_available_playbooks
    echo ""
    echo -e "Environment variables:\n"
    echo -e "  --- AWS Credentials ---"
    echo -e "  AWS_ACCESS_KEY_ID"
    echo -e "  AWS_PRIVATE_ACCESS_KEY"
    echo -e "  AWS_SESSION_TOKEN"
    echo -e "  AWS_PROFILE"
    echo -e "  AWS_REGION"
    echo -e ""
    echo -e "Volumes:\n"
    echo -e "  /ansible/.ssh/\t\tSSH keys to be used for bootstrapping"
    echo -e "  /ansible/.aws/\t\tAWS credentials file, if any"
    echo -e "  /ansible/inventory/\t\tInventory files here"
    echo -e "  /ansible/group_vars/\t\tGroup var files here"
    echo -e "  /ansible/host_vars/\t\tHost var files here"
    echo -e ""
}

TARGET="$1"
PLAYBOOK="playbooks/$TARGET.yml"

if [ -z "$TARGET" ]; then
    help
    exit 0
fi

if [ ! -f "$PLAYBOOK" ]; then
    echo "Unknown playbook: $TARGET"
    print_available_playbooks
    exit 1
fi

ansible-playbook -i /ansible/inventory "$PLAYBOOK"
