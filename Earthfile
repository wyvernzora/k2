VERSION 0.8

ansible:
    FROM DOCKERFILE -f containers/ansible/Dockerfile .
    SAVE IMAGE ghcr.io/wyvernzora/k2-ansible

manifests:
    FROM DOCKERFILE -f containers/builder/Dockerfile .
    COPY . .
    RUN npm ci && npm run build
    SAVE ARTIFACT deploy AS LOCAL deploy

diff:
    FROM DOCKERFILE -f containers/builder/Dockerfile .
    RUN git clone --no-checkout -b deploy --single-branch --depth 2 https://github.com/wyvernzora/k2
    COPY (+manifests/deploy) k2/
    RUN (cd k2 && git reset . && git diff origin/deploy) | tee deploy.diff
    SAVE ARTIFACT deploy.diff AS LOCAL deploy.diff
