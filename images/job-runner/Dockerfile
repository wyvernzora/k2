# syntax=docker/dockerfile:1.25
ARG ALPINE_VERSION=3.24
FROM alpine:${ALPINE_VERSION}

# renovate: datasource=github-releases depName=kubernetes/kubernetes
ARG KUBECTL_VERSION=v1.36.2
# renovate: datasource=github-releases depName=homeport/dyff extractVersion=^v(?<version>.+)$
ARG DYFF_VERSION=1.12.0

RUN apk add --no-cache \
    bash \
    bind-tools \
    ca-certificates \
    coreutils \
    curl \
    diffutils \
    findutils \
    gawk \
    gettext \
    grep \
    gzip \
    jq \
    netcat-openbsd \
    openssl \
    py3-click \
    py3-jsonschema \
    py3-kubernetes \
    py3-requests \
    py3-rich \
    py3-websockets \
    py3-yaml \
    python3 \
    sed \
    tar \
    unzip \
    wget \
    yq-go

RUN set -eux; \
    case "$(apk --print-arch)" in \
      x86_64) kubectl_arch=amd64 ;; \
      aarch64) kubectl_arch=arm64 ;; \
      *) echo "unsupported kubectl architecture: $(apk --print-arch)" >&2; exit 1 ;; \
    esac; \
    kubectl_url="https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${kubectl_arch}/kubectl"; \
    curl -fsSLo /tmp/kubectl "${kubectl_url}"; \
    curl -fsSLo /tmp/kubectl.sha256 "${kubectl_url}.sha256"; \
    echo "$(cat /tmp/kubectl.sha256)  /tmp/kubectl" | sha256sum -c -; \
    install -m 0755 /tmp/kubectl /usr/local/bin/kubectl; \
    rm /tmp/kubectl /tmp/kubectl.sha256; \
    kubectl version --client=true

RUN set -eux; \
    case "$(apk --print-arch)" in \
      x86_64) dyff_arch=amd64 ;; \
      aarch64) dyff_arch=arm64 ;; \
      *) echo "unsupported dyff architecture: $(apk --print-arch)" >&2; exit 1 ;; \
    esac; \
    dyff_archive="dyff_${DYFF_VERSION}_linux_${dyff_arch}.tar.gz"; \
    dyff_base_url="https://github.com/homeport/dyff/releases/download/v${DYFF_VERSION}"; \
    curl -fsSLo "/tmp/${dyff_archive}" "${dyff_base_url}/${dyff_archive}"; \
    curl -fsSLo /tmp/dyff-checksums.txt "${dyff_base_url}/checksums.txt"; \
    cd /tmp; \
    grep " ${dyff_archive}$" dyff-checksums.txt | sha256sum -c -; \
    tar -xzf "/tmp/${dyff_archive}" -C /usr/local/bin dyff; \
    chmod 0755 /usr/local/bin/dyff; \
    rm "/tmp/${dyff_archive}" /tmp/dyff-checksums.txt; \
    dyff version

COPY smoke-test.sh /usr/local/bin/k2-job-runner-smoke-test
RUN chmod 0755 /usr/local/bin/k2-job-runner-smoke-test \
    && adduser -D -u 65532 runner \
    && /usr/local/bin/k2-job-runner-smoke-test

USER 65532:65532
WORKDIR /workspace
ENTRYPOINT ["/bin/bash"]
CMD ["-lc", "k2-job-runner-smoke-test"]
