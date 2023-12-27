FROM alpine AS builder
RUN apk add --no-cache curl git kustomize
WORKDIR /crds

# renovate: datasource=github-releases depName=argoproj/argo-cd
ARG ARGOCD_CRD_VERSION=v2.9.3
RUN kustomize build https://github.com/argoproj/argo-cd/manifests/crds?ref=${ARGOCD_CRD_VERSION} > argocd.crds.yaml

# renovate: datasource=github-releases depName=cert-manager/cert-manager
ARG CERT_MANAGER_CRD_VERSION=v1.12.2
RUN curl -sL https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_CRD_VERSION}/cert-manager.crds.yaml > cert-manager.crds.yaml

# renovate: datasource=github-releases depName=metallb/metallb
ARG METALLB_CRD_VERSION=v0.13.12
RUN kustomize build https://github.com/metallb/metallb/config/crd?ref=${METALLB_CRD_VERSION} > metallb.crds.yaml

# renovate: datasource=github-releases depName=1Password/onepassword-operator
ARG ONEPASS_CRD_VERSION=v1.8.0
RUN kustomize build https://github.com/1Password/onepassword-operator/config/crd?ref=${ONEPASS_CRD_VERSION} > 1password.crds.yaml

# renovate: datasource=github-releases depName=traefik/traefik-helm-chart
ARG TRAEFIK_CRD_VERSION=v26.0.0
RUN kustomize build https://github.com/traefik/traefik-helm-chart/traefik/crds/?ref=${TRAEFIK_CRD_VERSION} > traefik.crds.yaml

FROM scratch
COPY --from=builder /crds /crds
