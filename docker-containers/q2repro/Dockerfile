FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS go-builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
COPY internal/ ./internal/
COPY cmd/ ./cmd/
RUN GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /insta-game ./cmd/insta-game

FROM debian:bookworm-slim AS q2repro-builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    build-essential \
    git \
    meson \
    ninja-build \
    pkg-config \
    libsdl2-dev \
    libcurl4-openssl-dev \
    libjpeg-dev \
    libopenal-dev \
    libpng-dev \
    libvorbis-dev \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --recurse-submodules https://github.com/Paril/q2repro.git /src/q2repro

WORKDIR /src/q2repro

RUN meson setup builddir \
    && meson compile -C builddir \
    && install -m 0755 "$(find builddir -type f \( -name q2reproded -o -name q2proded -o -name q2pro \) -perm /111 | head -n 1)" /usr/local/bin/q2proded \
    && mkdir -p /usr/local/share/baseq2 \
    && find builddir -type f -name 'game*.so' -exec install -m 0755 {} /usr/local/share/baseq2/ \;

FROM debian:bookworm-slim

COPY --from=q2repro-builder /usr/local/bin/q2proded /usr/local/share/insta-game/seed/q2proded
COPY --from=q2repro-builder /usr/local/share/baseq2/ /usr/local/share/insta-game/seed/baseq2/

WORKDIR /usr/local/share/insta-game/seed

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libcurl4 \
    libjpeg62-turbo \
    libopenal1 \
    libsdl2-2.0-0 \
    libvorbisfile3 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /usr/local/share/insta-game/seed/baseq2 /usr/local/share/insta-game/seed/data \
    && useradd -r -s /bin/false game \
    && chown -R game:game /usr/local/share/insta-game/seed


COPY --from=go-builder /insta-game /usr/local/bin/insta-game

COPY docker-containers/q2repro/server.cfg /usr/local/share/insta-game/seed/default-server.cfg

EXPOSE 27910/udp
EXPOSE 5001/tcp

ENV DATA_SEED=/usr/local/share/insta-game/seed DATA_ROOT=/opt

RUN mkdir -p /opt && chown game:game /opt
WORKDIR /opt
USER game
ENTRYPOINT ["/usr/local/bin/insta-game", "agent"]
