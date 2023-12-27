FROM alpine AS builder

RUN apk add --no-cache curl git kustomize
WORKDIR /crds

# renovate: datasource=github-releases depName=traefik/traefik-helm-chart
ARG TRAEFIK_CRD_VERSION=v26.0.0
RUN kustomize build https://github.com/traefik/traefik-helm-chart/traefik/crds/?ref=${TRAEFIK_CRD_VERSION} > traefik.crds.yaml

# renovate: datasource=github-releases depName=cert-manager/cert-manager
ARG CERT_MANAGER_CRD_VERSION=v1.12.2
RUN curl -sL https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_CRD_VERSION}/cert-manager.crds.yaml > cert-manager.crds.yaml

# renovate: datasource=github-releases depName=1Password/onepassword-operator
ARG ONEPASS_CRD_VERSION=v1.8.0
RUN kustomize build https://github.com/1Password/onepassword-operator/config/crd?ref=${ONEPASS_CRD_VERSION} > 1password.crds.yaml

FROM scratch
COPY --from=builder /crds /crds
COPY ./templates /templates
COPY ./run.sh .
