FROM node:alpine

RUN apk add --no-cache \
    bash \
    git \
    jq \
    helm \
    kustomize \
    yq

RUN  adduser -D builder
USER builder
WORKDIR /home/builder

ENV NX_DAEMON=false
RUN helm repo add 1password https://1password.github.io/connect-helm-charts && \
    helm repo add jetstack https://charts.jetstack.io && \
    helm repo add argo https://argoproj.github.io/argo-helm && \
    helm repo add metallb https://metallb.github.io/metallb && \
    helm repo add common https://technosophos.github.io/common-chart/ && \
    helm repo add traefik https://traefik.github.io/charts && \
    helm repo add authelia https://charts.authelia.com && \
    helm repo add ori-edge https://ori-edge.github.io/k8s_gateway && \
    helm repo add rancher-latest https://releases.rancher.com/server-charts/latest && \
    helm repo add kube-vip https://kube-vip.github.io/helm-charts && \
    helm repo add longhorn https://charts.longhorn.io && \
    helm repo add emberstack https://emberstack.github.io/helm-charts && \
    helm repo update
