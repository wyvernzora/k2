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

ansible:
    FROM DOCKERFILE -f ansible/Dockerfile ./ansible
    SAVE IMAGE --push $IMAGE_REPOSITORY/k2-ansible:${TAG}

bootstrap:
    FROM DOCKERFILE -f kairos/bootstrap/Dockerfile ./kairos/bootstrap
    SAVE IMAGE --push $IMAGE_REPOSITORY/k2-bootstrap:${TAG}
