FROM debian:bookworm-slim AS mayo

ARG MAYO_VERSION=0.10.0
ARG MAYO_SHA256=9efa2fda0f5ff261445e8db6ac1e976b8e4e809fc11dc66170c23689948d05c2

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp/mayo

RUN curl \
        --fail \
        --location \
        --show-error \
        --silent \
        --output "MayoConv-${MAYO_VERSION}-x86_64.AppImage" \
        "https://github.com/fougue/mayo/releases/download/v${MAYO_VERSION}/MayoConv-${MAYO_VERSION}-x86_64.AppImage" \
    && printf '%s  %s\n' \
        "${MAYO_SHA256}" \
        "MayoConv-${MAYO_VERSION}-x86_64.AppImage" \
        | sha256sum --check --strict \
    && chmod +x "MayoConv-${MAYO_VERSION}-x86_64.AppImage" \
    && "./MayoConv-${MAYO_VERSION}-x86_64.AppImage" \
        --appimage-extract \
        >/dev/null

FROM golang:1.27.1-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libfontconfig1 \
        libfreetype6 \
        libgl1 \
        libx11-6 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=mayo \
    /tmp/mayo/squashfs-root \
    /opt/mayo

ENV MAYO_EXECUTABLE=/opt/mayo/AppRun

RUN test -x "${MAYO_EXECUTABLE}"
