VERSION 0.6
FROM alpine
ARG IMAGE_REPOSITORY=ghcr.io/wyvernzora

# renovate: datasource=docker depName=renovate/renovate versioning=docker
ARG RENOVATE_VERSION=37
# renovate: datasource=docker depName=koalaman/shellcheck-alpine versioning=docker
ARG SHELLCHECK_VERSION=v0.9.0

###############################################################################
# Build & Test                                                                #
###############################################################################
renovate-validate:
    ARG RENOVATE_VERSION
    FROM renovate/renovate:$RENOVATE_VERSION
    WORKDIR /usr/src/app
    COPY ./.github/renovate.json .
    RUN renovate-config-validator

shellcheck:
    ARG SHELLCHECK_VERSION
    FROM koalaman/shellcheck-alpine:$SHELLCHECK_VERSION
    WORKDIR /mnt
    COPY . .
    RUN find . -name "*.sh" -name "!node_modules" -print | xargs -r -n1 shellcheck

test-apps:
    FROM node:alpine
    WORKDIR /mnt
    COPY . .
    RUN apk add --no-cache kustomize helm git && \
        npm install -g cdk8s-cli && \
        npm install
    RUN npm run lint:check && ls -la ./test
    RUN ./test/test-apps

test:
    BUILD +renovate-validate
    BUILD +shellcheck
    BUILD +test-apps

test-multiarch:
    BUILD --platform=linux/amd64 --platform=linux/arm64 +test

###############################################################################
# Ansible Playbooks                                                           #
###############################################################################
ansible:
    FROM willhallonline/ansible:2.15-alpine-3.18
    COPY ansible .
    RUN pip install --no-cache-dir botocore boto3 && \
        ansible-galaxy install -r requirements.yml
    VOLUME [ "/ansible/.ssh", "/ansible/.aws", "/ansible/inventory" ]
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
    COPY infrastructure/crds/*.yaml .
    RUN apk add --no-cache kustomize git && \
        kustomize build . > crds.yaml
    SAVE ARTIFACT crds.yaml

generate-crd-constructs:
    FROM node:alpine
    WORKDIR /build
    RUN mkdir -p /output && \
        npm install -g cdk8s-cli
    COPY infrastructure/crds /output
    COPY (+render-crd-manifests/crds.yaml) /build
    RUN cdk8s import -l typescript -o /output crds.yaml
    SAVE ARTIFACT /output AS LOCAL ./infrastructure/crds

###############################################################################
# Kairos Cluster Init                                                         #
###############################################################################
cluster-init-manifests:
    FROM alpine
    WORKDIR /build
    COPY kairos/cluster-init .
    COPY infrastructure/core/argocd/values.yaml argocd/values.yaml
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
