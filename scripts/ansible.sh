#!/bin/bash

ROOT=$(dirname $0)

docker run --rm -it \
    -e AWS_ACCESS_KEY_ID \
    -e AWS_SECRET_ACCESS_KEY \
    -e AWS_REGION \
    -e AWS_PROFILE \
    -v $HOME/.aws:/ansible/.aws:ro \
    -v $HOME/.ssh:/ansible/.ssh:ro \
    -v $ROOT/../.private/ansible-inventory:/ansible/inventory:ro \
    ghcr.io/wyvernzora/k2-ansible:latest "$@"
