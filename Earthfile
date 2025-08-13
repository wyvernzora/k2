VERSION 0.8

ansible:
    FROM DOCKERFILE -f containers/ansible/Dockerfile .
    SAVE IMAGE ghcr.io/wyvernzora/k2-ansible

build-image-base:
    ARG TAG="latest"
    FROM ./build+image
    SAVE IMAGE ghcr.io/wyvernzora/k2-build:${TAG}

build-image:
    BUILD --platform=linux/amd64 --platform=linux/arm64 +build-image-base

for-all:
    ARG TARGET="no-op"
    LOCALLY
    WAIT
        FOR dir IN $(ls -d apps/* 2>/dev/null)
            BUILD "./$dir+$TARGET"
        END
    END

crd-constructs:
    BUILD +for-all --TARGET="crd-constructs"

manifests:
    ARG TAG="latest"
    FROM ghcr.io/wyvernzora/k2-build:${TAG}

    # Cache NPM dependencies as part of the image
    COPY package.json package-lock.json ./
    RUN npm ci

    # Copy the data
    COPY . .
    BUILD +crd-constructs
    RUN /scripts/v2/synthesize-app-manifests.sh
    SAVE ARTIFACT deploy AS LOCAL deploy

diff:
    ARG TAG="latest"
    FROM ghcr.io/wyvernzora/k2-build:${TAG}
    RUN git clone --no-checkout -b deploy --single-branch --depth 2 https://github.com/wyvernzora/k2
    COPY (+manifests/deploy) k2/
    RUN (cd k2 && git add . && git diff --cached origin/deploy) | tee deploy.diff
    SAVE ARTIFACT deploy.diff AS LOCAL deploy.diff
