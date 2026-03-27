FROM mcr.microsoft.com/devcontainers/base:bookworm

ARG GO_VERSION=1.26.0
ARG TARGETARCH

ENV PATH="/usr/local/go/bin:${PATH}"

# Install build tools required by CGO/C++17 sources in sources/misc/prefixtree.
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        g++ \
        gcc \
        make \
        cmake \
        gdb \
        pkg-config \
        git \
        tar \
    && case "${TARGETARCH}" in \
        "amd64") GO_ARCH="amd64" ;; \
        "arm64") GO_ARCH="arm64" ;; \
        *) echo "Unsupported TARGETARCH: ${TARGETARCH}" && exit 1 ;; \
    esac \
    && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -o /tmp/go.tar.gz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm -f /tmp/go.tar.gz \
    && go version \
    && rm -rf /var/lib/apt/lists/*
