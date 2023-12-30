FROM alpine AS helm

WORKDIR /build
COPY . .
RUN mkdir -p /manifests
RUN apk add --no-cache helm
RUN helm repo add argo https://argoproj.github.io/argo-helm && \
    helm repo add 1password https://1password.github.io/connect-helm-charts && \
    helm repo update

RUN cat argocd/namespace.yaml > /manifests/argocd.yaml && \
    helm template \
      --namespace argocd \
      --create-namespace \
      --release-name argocd \
      argo/argo-cd >> /manifests/argocd.yaml && \
    cat argocd/root-app.yaml >> /manifests/argocd.yaml

RUN cat 1password/namespace.yaml > /manifests/1password.yaml && \
    cat 1password/secret.yaml >> /manifests/1password.yaml && \
    helm template \
      --namespace 1password \
      --create-namespace \
      --release-name 1password-connect \
      --set operator.create=true \
      1password/connect >> /manifests/1password.yaml

FROM alpine
COPY --from=helm /manifests /manifests
COPY ./run.sh .
