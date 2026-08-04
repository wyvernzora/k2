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
  Usage: docker run [options] k2-ansible:local <playbook-name> [ansible-playbook options]

  $(print_available_playbooks)

  Volumes:

    /root/.ssh/           SSH keys to be used for bootstrapping
    /ansible/inventory/   Inventory files (read-only is sufficient)
    /ansible/credentials/ Generated local credentials (read/write)
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

shift
export ANSIBLE_ROLES_PATH="$ANSIBLE_ROLES_PATH:/ansible/roles"
export ANSIBLE_HOST_KEY_CHECKING="False"
exec ansible-playbook -i /ansible/inventory "$PLAYBOOK" "$@"
