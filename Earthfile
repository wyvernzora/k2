VERSION 0.8

ansible:
    FROM DOCKERFILE -f containers/ansible/Dockerfile .
    SAVE IMAGE ghcr.io/wyvernzora/k2-ansible

manifests:
    FROM DOCKERFILE -f containers/builder/Dockerfile .
    COPY . .
    RUN npm ci && npm run build
    SAVE ARTIFACT deploy/* AS LOCAL deploy/
