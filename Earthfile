VERSION 0.8

ansible:
    FROM DOCKERFILE -f containers/ansible/Dockerfile .
    SAVE IMAGE ghcr.io/wyvernzora/k2-ansible

build-image:
    BUILD ./build+image

manifests:
    FROM ghcr.io/wyvernzora/k2-build:latest
    COPY . .
    RUN npm ci && npm run build
    SAVE ARTIFACT deploy AS LOCAL deploy

diff:
    FROM ghcr.io/wyvernzora/k2-build:latest
    RUN git clone --no-checkout -b deploy --single-branch --depth 2 https://github.com/wyvernzora/k2
    COPY (+manifests/deploy) k2/
    RUN (cd k2 && git add . && git diff --cached origin/deploy) | tee deploy.diff
    SAVE ARTIFACT deploy.diff AS LOCAL deploy.diff
