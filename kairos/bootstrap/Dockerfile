FROM alpine AS helm

WORKDIR /build
COPY . .
RUN mkdir -p /manifests
RUN apk add --no-cache helm yq
RUN helm repo add argo https://argoproj.github.io/argo-helm && helm repo update
RUN cat namespace.yaml > /manifests/k2-bootstrap.yaml
RUN helm template \
    --namespace argocd \
    --create-namespace \
    --release-name argocd \
      argo/argo-cd >> /manifests/k2-bootstrap.yaml 
RUN cat root.yaml >> /manifests/k2-bootstrap.yaml

FROM alpine
COPY --from=helm /manifests /manifests
COPY ./run.sh .
