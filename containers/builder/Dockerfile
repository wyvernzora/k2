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
