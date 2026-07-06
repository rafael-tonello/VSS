
FROM mcr.microsoft.com/devcontainers/go:1-1.24-bookworm

# Native toolchain/libs needed by CGO + C++ modules used in this repository.
RUN set -eux; \
    for src in /etc/apt/sources.list /etc/apt/sources.list.d/*.list; do \
        [ -f "$src" ] || continue; \
        sed -i '/dl\.yarnpkg\.com/d' "$src"; \
    done; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        build-essential \
        g++ \
        cmake \
        pkg-config \
        libssl-dev; \
    rm -rf /var/lib/apt/lists/*
