VERSION 0.8

ansible:
    FROM DOCKERFILE -f containers/ansible/Dockerfile .
    SAVE IMAGE ghcr.io/wyvernzora/k2-ansible

argocd-plugin:
    FROM DOCKERFILE -f containers/argocd-plugin/Dockerfile .
    SAVE IMAGE ghcr.io/wyvernzora/k2-argocd-plugin

manifests:
    FROM DOCKERFILE -f containers/builder/Dockerfile .
    COPY . .
    RUN npm ci && npm run build
    SAVE ARTIFACT deploy/dist/* AS LOCAL deploy/dist/
