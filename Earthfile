VERSION 0.6
FROM alpine
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
        py-boto3 && \
        adduser -D -h '/ansible' ansible && \
        rm -rf /var/cache/apk/* /usr/share/doc /usr/share/man/ /usr/share/info/* /var/cache/man/* /tmp/* /etc/fstab && \
        rm -fr /etc/init.d /lib/rc /etc/conf.d /etc/inittab /etc/runlevels /etc/rc.conf && \
        rm -rf /etc/sysctl* /etc/modprobe.d /etc/modules /etc/mdev.conf /etc/acpi && \
        find / -xdev -type l -exec test ! -e {} \; -delete
    WORKDIR '/ansible'
    USER ansible
    COPY ansible .
    RUN ansible-galaxy install -r requirements.yml
    ENV ANSIBLE_ROLES_PATH="/ansible/roles:/usr/share/ansible/roles:/etc/ansible/roles"
    VOLUME [ "/ansible/.ssh", "/ansible/.aws", "/ansible/group_vars", "/ansible/host_vars", "/ansible/inventory" ]
    ENTRYPOINT [ "/ansible/entrypoint.sh" ]
    SAVE IMAGE $IMAGE_REPOSITORY/k2-ansible:dev

ansible-multiarch:
    BUILD --platform=linux/amd64 --platform=linux/arm64 +ansible

###############################################################################
# Generate cdk8s constructs for imported CRDs                                 #
###############################################################################
render-crd-manifests:
    FROM alpine
    WORKDIR /build
    COPY crds/kustomization.yaml .
    RUN apk add --no-cache kustomize git && \
        kustomize build . > crds.yaml
    SAVE ARTIFACT crds.yaml

generate-crd-constructs:
    FROM node:alpine
    WORKDIR /build
    COPY crds/*.yaml .
    COPY (+render-crd-manifests/crds.yaml) /build
    RUN mkdir -p /imports && \
        npm install -g cdk8s-cli && \
        cdk8s import -l typescript -o /imports crds.yaml && \
        cp /build/kustomization.yaml /build/k2-app.yaml /imports/
    SAVE ARTIFACT /imports AS LOCAL ./crds

###############################################################################
# Kairos Cluster Init                                                         #
###############################################################################
cluster-init-manifests:
    FROM alpine
    WORKDIR /build
    COPY kairos/cluster-init .
    COPY gitops/core/argocd/values.yaml argocd/values.yaml
    COPY (+render-crd-manifests/crds.yaml) crds.yaml
    RUN ./build.sh
    SAVE ARTIFACT /manifests

cluster-init:
    FROM alpine
    COPY +cluster-init-manifests/manifests /manifests
    COPY kairos/cluster-init/run.sh .
    SAVE IMAGE $IMAGE_REPOSITORY/k2-cluster-init:dev

cluster-init-multiarch:
    BUILD --platform=linux/amd64 --platform=linux/arm64 +cluster-init
