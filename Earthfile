VERSION 0.8
ARG TAG="latest"

#
# +job-runner-image: Creates the runtime utility image for K2 Jobs/init containers
#
job-runner-image:
    BUILD --platform=linux/amd64 --platform=linux/arm64 +job-runner-image-base

job-runner-image-base:
    ARG TAG="latest"
    # renovate: datasource=docker depName=alpine
    ARG ALPINE_VERSION="3.24.1"
    # renovate: datasource=github-releases depName=kubernetes/kubernetes
    ARG KUBECTL_VERSION="v1.36.2"
    # renovate: datasource=github-releases depName=homeport/dyff extractVersion=^v(?<version>.+)$
    ARG DYFF_VERSION="1.12.0"
    FROM DOCKERFILE --build-arg ALPINE_VERSION=$ALPINE_VERSION --build-arg KUBECTL_VERSION=$KUBECTL_VERSION --build-arg DYFF_VERSION=$DYFF_VERSION images/job-runner
    SAVE IMAGE ghcr.io/wyvernzora/k2-job-runner:${TAG}

#
# +crd-manifest: Extracts CRDs from an app's Helm chart dependencies into
# apps/<name>/crds/crds.k8s.yaml. Run per-app when introducing a new chart
# or bumping its version. Output is committed; +crd-constructs reads it.
#
# Usage: earthly +crd-manifest --APP_ROOT=apps/<name>
#
crd-manifest:
    ARG TAG="latest"
    ARG APP_ROOT
    FROM ./build+image
    COPY ./tools+k2-tools-cli/k2-tools /usr/local/bin/k2-tools
    COPY . .
    RUN k2-tools build crd-manifest "$APP_ROOT"
    SAVE ARTIFACT $APP_ROOT/crds/crds.k8s.yaml AS LOCAL $APP_ROOT/crds/crds.k8s.yaml

#
# +crd-constructs: Generates TypeScript cdk8s constructs for each app based on its CRD manifest
#
crd-constructs:
    LOCALLY
    WAIT
        FOR APP_ROOT IN $(ls -d apps/*/crds/crds.k8s.yaml 2>/dev/null | sed 's#/crds/crds.k8s.yaml$##')
            BUILD +crd-constructs-base --APP_ROOT=$APP_ROOT
        END
    END

crd-constructs-base:
    ARG TAG="latest"
    ARG APP_ROOT
    FROM ./build+image
    COPY ./tools+k2-tools-cli/k2-tools /usr/local/bin/k2-tools
    COPY . .
    RUN k2-tools build crd-constructs "$APP_ROOT"
    SAVE ARTIFACT $APP_ROOT/crds/*.ts AS LOCAL $APP_ROOT/crds/

#
# +app-manifests: Generates k8s deployment manifests
#
k8s-manifests:
    ARG TAG="latest"
    FROM --pass-args +npm-install
    COPY ./tools+k2-tools-cli/k2-tools /usr/local/bin/k2-tools
    COPY . .
    RUN rm -rf deploy
    RUN k2-tools build crd-constructs
    RUN k2-tools build manifests
    SAVE ARTIFACT deploy AS LOCAL deploy

#
# +diff-manifests: diff your newly-built deploy/ against the remote 'deploy' branch
#
diff-manifests:
    ARG TAG="latest"
    FROM ./build+image
    COPY ./tools+k2-tools-cli/k2-tools /usr/local/bin/k2-tools
    COPY . .
    RUN k2-tools build diff-manifests https://github.com/wyvernzora/k2.git
    SAVE ARTIFACT deploy-diff.md AS LOCAL deploy-diff.md

#
# +lint: lints codebase
#
lint:
    ARG TAG="latest"
    FROM --pass-args +npm-install
    COPY ./tools+k2-tools-cli/k2-tools /usr/local/bin/k2-tools
    COPY . .
    RUN k2-tools build lint

npm-install:
    ARG TAG="latest"
    FROM ./build+image
    COPY package.json package-lock.json ./
    RUN NO_UPDATE_NOTIFIER=true npm ci

#
# +kairos-image-build-unit: tests and validates the root k2-tools image builder
#
kairos-image-build-unit:
    FROM golang:1.26-bookworm
    WORKDIR /src
    COPY tools/go.mod tools/go.sum ./tools/
    RUN --mount=type=cache,target=/go/pkg/mod cd tools && go mod download
    COPY tools ./tools
    COPY kairos/Dockerfile ./kairos/Dockerfile
    COPY kairos/overlays ./kairos/overlays
    COPY kairos/scripts ./kairos/scripts
    COPY kairos/tools/vm-presets ./kairos/tools/vm-presets
    COPY kairos/tools/e2e-scenarios ./kairos/tools/e2e-scenarios
    COPY kairos/targets.yaml kairos/versions.env ./kairos/
    COPY kairos/node-agent ./kairos/node-agent
    RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build cd tools && go test ./...
    RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build cd tools && go vet ./...
    RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build cd tools && CGO_ENABLED=0 go build -o k2-tools ./cmd/k2-tools
    RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build cd /src/kairos/node-agent && go test ./...
    RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build cd /src/kairos/node-agent && go vet ./...
    RUN /src/tools/k2-tools --repo-root /src image --build-root /src/kairos --kairos-root /src/kairos plan --all --format json >/tmp/kairos-image-build-plans.json

#
# +kairos-image-build-cli: builds the k2-tools CLI used by Earthly artifact targets
#
kairos-image-build-cli:
    FROM --pass-args +kairos-image-build-unit
    WORKDIR /src
    SAVE ARTIFACT /src/tools/k2-tools

#
# +kairos-image-build-artifact: builds, patches, inspects, and exports Kairos boot artifacts in Linux
#
kairos-image-build-artifact:
    ARG KAIROS_TARGET="ubuntu-26.04-arm64-rpi4cb-k8s"
    FROM earthly/dind:ubuntu-23.04-docker-25.0.2-1
    WORKDIR /src
    RUN command -v bash && command -v xz && command -v docker && command -v dockerd
    COPY kairos/Dockerfile ./kairos/Dockerfile
    COPY kairos/overlays ./kairos/overlays
    COPY kairos/node-agent ./kairos/node-agent
    COPY kairos/targets.yaml kairos/versions.env ./kairos/
    COPY +kairos-image-build-cli/k2-tools /usr/local/bin/k2-tools
    WITH DOCKER
        RUN --mount=type=cache,target=/var/cache/k2-kairos-image set -eu; \
            docker run --privileged --rm tonistiigi/binfmt:latest --install all; \
            docker buildx rm k2-earthly-builder >/dev/null 2>&1 || true; \
            docker buildx create --name k2-earthly-builder --driver docker-container --use >/dev/null; \
            docker buildx inspect --bootstrap >/dev/null; \
            cache_root="/var/cache/k2-kairos-image/oci/${KAIROS_TARGET}"; \
            cache_current="${cache_root}/current"; \
            cache_next="${cache_root}/next"; \
            mkdir -p "${cache_current}"; \
            rm -rf "${cache_next}"; \
            mkdir -p "${cache_next}"; \
            cache_from_args=""; \
            if [ -f "${cache_current}/index.json" ]; then \
                cache_from_args="--cache-from type=local,src=${cache_current}"; \
            fi; \
            k2-tools --repo-root /src image --build-root /src/kairos --kairos-root /src/kairos --artifacts /out build oci \
                ${cache_from_args} \
                --cache-to "type=local,dest=${cache_next},mode=max" \
                "$KAIROS_TARGET"; \
            rm -rf "${cache_current}"; \
            mv "${cache_next}" "${cache_current}"; \
            k2-tools --repo-root /src image --build-root /src/kairos --kairos-root /src/kairos --artifacts /out inspect oci "$KAIROS_TARGET"; \
            k2-tools --repo-root /src image --build-root /src/kairos --kairos-root /src/kairos --artifacts /out build artifact "$KAIROS_TARGET"; \
            k2-tools --repo-root /src image --build-root /src/kairos --kairos-root /src/kairos --artifacts /out inspect artifact "$KAIROS_TARGET"
    END
    SAVE ARTIFACT /out/$KAIROS_TARGET AS LOCAL kairos/artifacts/$KAIROS_TARGET
