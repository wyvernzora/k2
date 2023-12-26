VERSION 0.6
FROM alpine
ARG VERSION="latest"
ARG IMAGE_REPOSITORY=ghcr.io/wyvernzora

# renovate: datasource=docker depName=renovate/renovate versioning=docker
ARG RENOVATE_VERSION=37
# renovate: datasource=docker depName=koalaman/shellcheck-alpine versioning=docker
ARG SHELLCHECK_VERSION=v0.9.0

version:
    FROM alpine
    RUN apk add git
    COPY . ./
    RUN echo $(git describe --exact-match --tags || echo "$(git log --oneline -n 1 | cut -d" " -f1)") > VERSION
    SAVE ARTIFACT VERSION VERSION

build:
    COPY +version/VERSION ./
    ARG VERSION=$(cat VERSION)
    FROM DOCKERFILE -f bundles/${BUNDLE}/Dockerfile ./bundles/${BUNDLE}
    SAVE IMAGE --push $IMAGE_REPOSITORY/k2-kairos-bundle:${BUNDLE}

renovate-validate:
    ARG RENOVATE_VERSION
    FROM renovate/renovate:$RENOVATE_VERSION
    WORKDIR /usr/src/app
    COPY ../.github/renovate.json .
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
    BUILD ./ansible+build

kairos-bootstrap:
    BUILD ./kairos/bootstrap+build
