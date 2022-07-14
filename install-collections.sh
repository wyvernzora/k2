#!/bin/bash
BASEDIR=$(dirname "$(readlink -f $0)")

mkdir -p "$HOME/.ansible/collections/ansible_collections"
ln -s "${BASEDIR}/collections" "$HOME/.ansible/collections/ansible_collections/wyvernzora"
