# Reproducible toolchain for Speed News.
#   target "dev":         Go + Node + linters. Used by the Dev Container for daily work (UI in the browser).
#   target "linux-build": dev + Rust + Tauri system libraries. Compiles the desktop app and Linux bundles.
# Versions are pinned here and mirrored in .mise.toml, rust-toolchain.toml and .github/workflows.

ARG GO_VERSION=1.27.4
FROM golang:${GO_VERSION}-bookworm AS dev

ARG NODE_VERSION=22.23.5
ARG GOLANGCI_LINT_VERSION=2.15.0
ARG USERNAME=developer
ARG USER_UID=1000
ARG USER_GID=1000

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git make sudo xz-utils \
    && rm -rf /var/lib/apt/lists/*

# Node from the official tarball (exact version, no third-party apt repository).
RUN architecture="$(dpkg --print-architecture)" \
    && case "${architecture}" in amd64) nodeArchitecture=x64 ;; arm64) nodeArchitecture=arm64 ;; *) echo "unsupported ${architecture}" && exit 1 ;; esac \
    && curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${nodeArchitecture}.tar.xz" \
       | tar -xJ -C /usr/local --strip-components=1 \
    && node --version

RUN curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
       | sh -s -- -b /usr/local/bin "v${GOLANGCI_LINT_VERSION}" \
    && golangci-lint --version

# Non-root user with the host UID, so files created in the bind mount keep the right owner.
RUN groupadd --gid "${USER_GID}" "${USERNAME}" \
    && useradd --uid "${USER_UID}" --gid "${USER_GID}" --create-home --shell /bin/bash "${USERNAME}" \
    && echo "${USERNAME} ALL=(root) NOPASSWD:ALL" > "/etc/sudoers.d/${USERNAME}" \
    && mkdir -p /go/pkg && chown -R "${USERNAME}:${USERNAME}" /go

USER ${USERNAME}
ENV GOPATH=/go GOFLAGS=-buildvcs=false
WORKDIR /workspace


FROM dev AS linux-build

ARG RUST_VERSION=1.100.0

USER root
# Tauri v2 prerequisites for Linux: https://v2.tauri.app/start/prerequisites/
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       build-essential file wget pkg-config \
       libwebkit2gtk-4.1-dev libxdo-dev libssl-dev libayatana-appindicator3-dev librsvg2-dev \
    && rm -rf /var/lib/apt/lists/*

USER developer
ENV RUSTUP_HOME=/home/developer/.rustup CARGO_HOME=/home/developer/.cargo
ENV PATH=/home/developer/.cargo/bin:${PATH}
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
       | sh -s -- -y --profile minimal --default-toolchain "${RUST_VERSION}" \
    && rustc --version
