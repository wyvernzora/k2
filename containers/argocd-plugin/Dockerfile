FROM node:alpine
ARG CONTAINER_ROOT="containers/argocd-plugin"

# Set up dependencies and tools
RUN apk add --no-cache bash git helm kustomize jq yq

# Run as user 999 as required by ArgoCD
RUN  adduser -DSu 999 argocd
USER argocd

# Copy plugin manifest as required by ArgoCD
WORKDIR /home/argocd/cmp-server/config/
COPY $CONTAINER_ROOT/scripts /home/argocd/k2/
COPY $CONTAINER_ROOT/plugin.yaml ./
