VERSION 0.8
ARG TAG="latest"

#
# +ansible-images: Creates the ansible playbook image
#
ansible-image:
    BUILD --platform=linux/amd64 --platform=linux/arm64 +ansible-image-base

ansible-image-base:
    ARG TAG="latest"
    FROM ./ansible+image
    SAVE IMAGE ghcr.io/wyvernzora/k2-ansible:${TAG}

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
    FROM +npm-install --TAG=$TAG
    COPY . .
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

npm-install:
    ARG TAG="latest"
    FROM ghcr.io/wyvernzora/k2-build:${TAG}
    COPY package.json package-lock.json ./
    RUN npm ci
