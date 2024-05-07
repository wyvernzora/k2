FROM node:alpine

# Set up dependencies and tools
RUN apk add --no-cache jq yq helm

# Run as user 999 as required by ArgoCD
RUN  adduser -DSu 999 argocd
USER argocd

# Copy plugin manifest as required by ArgoCD
WORKDIR /home/argocd/cmp-server/config/
COPY containers/argocd-plugin/plugin.yaml ./
