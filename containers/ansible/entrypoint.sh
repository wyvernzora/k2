#!/bin/sh

print_available_playbooks() {
    echo "Available playbooks:"
    echo ""
    for FILE in playbooks/*.yml; do
        echo "  $(basename "${FILE%.yml}")      $(head -n 1 "$FILE" | sed "s/^# //")"
    done
}

help() {
  echo "
  Usage: docker run [options] ghcr.io/wyvernzora/k2-ansible <playbook-name>

  $(print_available_playbooks)

  Environment variables:

    --- AWS Credentials ---
    AWS_ACCESS_KEY_ID
    AWS_PRIVATE_ACCESS_KEY
    AWS_SESSION_TOKEN
    AWS_PROFILE
    AWS_REGION

  Volumes:

    /root/.ssh/           SSH keys to be used for bootstrapping
    /root/.aws/           AWS credentials file, if any
    /ansible/inventory/   Inventory files here
  "
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

export ANSIBLE_ROLES_PATH="$ANSIBLE_ROLES_PATH:/ansible/roles"
export ANSIBLE_HOST_KEY_CHECKING="False"
ansible-playbook -i /ansible/inventory "$PLAYBOOK"
