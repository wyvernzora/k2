VERSION 0.8
ARG TAG="latest"

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
    LOCALLY
    WAIT
        FOR dir IN $(ls -d apps/* 2>/dev/null)
            BUILD "./$dir+crd-constructs" --TAG=$TAG
        END
    END

#
# +app-manifests: Generates k8s deployment manifests
#
k8s-manifests:
    ARG TAG="latest"
    ARG APP_ROOT
    FROM +npm-install --TAG=$TAG
    COPY . .
    WAIT
        FOR APP_ROOT IN $(ls -d apps/* 2>/dev/null)
            RUN /scripts/synthesize-app-manifests.sh "$APP_ROOT"
        END
    END
    RUN /scripts/synthesize-argocd-manifest.sh
    SAVE ARTIFACT deploy/* AS LOCAL deploy/

build-image-base:
    ARG TAG="latest"
    FROM ./build+image
    SAVE IMAGE ghcr.io/wyvernzora/k2-build:${TAG}

npm-install:
    ARG TAG="latest"
    FROM ghcr.io/wyvernzora/k2-build:${TAG}
    COPY package.json package-lock.json ./
    RUN npm ci

ansible:
    FROM DOCKERFILE -f containers/ansible/Dockerfile .
    SAVE IMAGE ghcr.io/wyvernzora/k2-ansible
