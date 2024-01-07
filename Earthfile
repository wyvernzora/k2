VERSION 0.6
FROM alpine
ARG TAG="latest"
ARG IMAGE_REPOSITORY=ghcr.io/wyvernzora

# renovate: datasource=docker depName=renovate/renovate versioning=docker
ARG RENOVATE_VERSION=37
# renovate: datasource=docker depName=koalaman/shellcheck-alpine versioning=docker
ARG SHELLCHECK_VERSION=v0.9.0

renovate-validate:
    ARG RENOVATE_VERSION
    FROM renovate/renovate:$RENOVATE_VERSION
    WORKDIR /usr/src/app
    COPY ./.github/renovate.json .
    RUN renovate-config-validator

shellcheck-lint:
    ARG SHELLCHECK_VERSION
    FROM koalaman/shellcheck-alpine:$SHELLCHECK_VERSION
    WORKDIR /mnt
    COPY . .
    RUN find . -name "*.sh" -print | xargs -r -n1 shellcheck

lint:
    BUILD +renovate-validate
    BUILD +shellcheck-lint

###############################################################################
# Ansible Playbooks                                                           #
###############################################################################
ansible:
    FROM alpine
    RUN apk add --no-cache \
        ansible \
        aws-cli \
        ca-certificates \
        openssh-client \
        py-boto3
    RUN adduser -D -h '/ansible' ansible
    WORKDIR '/ansible'
    USER ansible
    COPY ansible .
    RUN ansible-galaxy install -r requirements.yml
    ENV ANSIBLE_ROLES_PATH="/ansible/roles:/usr/share/ansible/roles:/etc/ansible/roles"
    VOLUME [ "/ansible/.ssh", "/ansible/.aws", "/ansible/group_vars", "/ansible/host_vars", "/ansible/inventory" ]
    ENTRYPOINT [ "/ansible/entrypoint.sh" ]
    SAVE IMAGE --push $IMAGE_REPOSITORY/k2-ansible:${TAG}

###############################################################################
# Kairos Cluster Init                                                         #
###############################################################################
cluster-init-manifests:
    FROM alpine
    WORKDIR /build
    COPY kairos/cluster-init .
    COPY gitops/core/argocd/values.yaml argocd/values.yaml
    RUN ./build.sh
    SAVE ARTIFACT /manifests

cluster-init:
    FROM scratch
    COPY +cluster-init-manifests/manifests /manifests
    COPY kairos/cluster-init/run.sh .
    SAVE IMAGE --push $IMAGE_REPOSITORY/k2-cluster-init:${TAG}
