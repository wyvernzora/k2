VERSION 0.8
ARG TAG="latest"

#
# +build-image: Creates the base image for all cdk8s builds
#
build-image:
    BUILD --platform=linux/amd64 --platform=linux/arm64 +build-image-base

build-image-base:
    ARG TAG="latest"
    FROM ./build+image
    SAVE IMAGE ghcr.io/wyvernzora/k2-build:${TAG}

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
    FROM ghcr.io/wyvernzora/k2-build:${TAG}
    COPY . .
    RUN build/scripts/generate-crd-manifest.sh "$APP_ROOT"
    SAVE ARTIFACT $APP_ROOT/crds/crds.k8s.yaml AS LOCAL $APP_ROOT/crds/crds.k8s.yaml

#
# +crd-constructs: Generates TypeScript cdk8s constructs for each app based on its CRD manifest
#
crd-constructs:
    LOCALLY
    WAIT
        FOR APP_ROOT IN $(ls -d apps/*/crds/crds.k8s.yaml 2>/dev/null | sed 's#/crds/crds.k8s.yaml$##')
            BUILD --pass-args +crd-constructs-base --APP_ROOT=$APP_ROOT
        END
    END

crd-constructs-base:
    ARG TAG="latest"
    ARG APP_ROOT
    FROM ghcr.io/wyvernzora/k2-build:${TAG}
    COPY . .
    RUN build/scripts/generate-crd-constructs.sh "$APP_ROOT"
    SAVE ARTIFACT $APP_ROOT/crds/*.ts AS LOCAL $APP_ROOT/crds/

#
# +app-manifests: Generates k8s deployment manifests
#
k8s-manifests:
    ARG TAG="latest"
    FROM --pass-args +npm-install
    COPY . .
    RUN rm -rf deploy
    RUN for APP_ROOT in $(ls -d apps/*/crds/crds.k8s.yaml 2>/dev/null | sed 's#/crds/crds.k8s.yaml$##'); do build/scripts/generate-crd-constructs.sh "$APP_ROOT"; done
    RUN npx tsx build/scripts/synthesize-manifests.ts
    SAVE ARTIFACT deploy AS LOCAL deploy

#
# +diff-manifests: diff your newly-built deploy/ against the remote 'deploy' branch
#
diff-manifests:
    ARG TAG="latest"
    FROM ghcr.io/wyvernzora/k2-build:${TAG}
    COPY . .
    RUN build/scripts/diff-manifests.sh https://github.com/wyvernzora/k2.git > deploy-diff.md
    SAVE ARTIFACT deploy-diff.md AS LOCAL deploy-diff.md

#
# +lint: lints codebase
#
lint:
    ARG TAG="latest"
    FROM --pass-args +npm-install
    COPY . .
    RUN for APP_ROOT in $(ls -d apps/*/crds/crds.k8s.yaml 2>/dev/null | sed 's#/crds/crds.k8s.yaml$##'); do build/scripts/generate-crd-constructs.sh "$APP_ROOT"; done
    RUN npx tsc --noEmit
    RUN npm run test:eslint-rules
    RUN npx eslint

npm-install:
    ARG TAG="latest"
    FROM ghcr.io/wyvernzora/k2-build:${TAG}
    COPY package.json package-lock.json ./
    RUN NO_UPDATE_NOTIFIER=true npm ci
