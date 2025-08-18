VERSION 0.8
ARG TAG="latest"

#
# +ansible-images: Creates the ansible playbook image
#
ansible-image:
    BUILD --platform=linux/amd64 --platform=linux/arm64 +ansible-image-base

#
# +build-image: Creates the base image for all cdk8s builds
#
build-image:
    BUILD --platform=linux/amd64 --platform=linux/arm64 +build-image-base

#
# +crd-constructs: Generates TypeScript cdk8s constructs for each app based on its CRD manifest
#
crd-constructs:
    ARG TAG="latest"
    FROM ghcr.io/wyvernzora/k2-build:${TAG}
    COPY . .
    FOR APP_ROOT IN $(ls -d apps/*/crds/crds.k8s.yaml 2>/dev/null | sed 's#/crds/crds.k8s.yaml$##')
        RUN /scripts/generate-crd-constructs.sh "$APP_ROOT"
        SAVE ARTIFACT $APP_ROOT/crds/*.ts AS LOCAL $APP_ROOT/crds/
    END

#
# +app-manifests: Generates k8s deployment manifests
#
k8s-manifests:
    ARG TAG="latest"
    ARG APP_ROOT
    FROM +npm-install --TAG=$TAG
    COPY . .
    FOR APP_ROOT IN $(ls -d apps/* 2>/dev/null)
        RUN /scripts/synthesize-app-manifests.sh "$APP_ROOT"
    END
    RUN /scripts/synthesize-argocd-manifest.sh
    SAVE ARTIFACT deploy/* AS LOCAL deploy/

build-image-base:
    ARG TAG="latest"
    FROM ./build+image
    SAVE IMAGE ghcr.io/wyvernzora/k2-build:${TAG}

ansible-image-base:
    ARG TAG="latest"
    FROM ./ansible+image
    SAVE IMAGE ghcr.io/wyvernzora/k2-ansible:${TAG}

npm-install:
    ARG TAG="latest"
    FROM ghcr.io/wyvernzora/k2-build:${TAG}
    COPY package.json package-lock.json ./
    RUN npm ci
