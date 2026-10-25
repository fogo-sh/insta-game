FROM --platform=$BUILDPLATFORM golang:1.27.3-bookworm AS go-builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
COPY internal/ ./internal/
COPY cmd/ ./cmd/
RUN GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /insta-game ./cmd/insta-game

FROM debian:bookworm-slim AS openarena-builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    git \
    libcurl4-openssl-dev \
    libogg-dev \
    libsdl2-dev \
    libvorbis-dev \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 https://github.com/OpenArena/engine.git /src/openarena-engine

WORKDIR /src/openarena-engine

RUN make -j"$(nproc)" \
    BUILD_SERVER=1 \
    BUILD_CLIENT=0 \
    BUILD_GAME_SO=0 \
    BUILD_GAME_QVM=0 \
    BUILD_STANDALONE=1 \
    BASEGAME=baseoa \
    MISSIONPACK=missionpack \
    && install -m 0755 build/release-linux-aarch64/oa_ded.aarch64 /openarena-ded

FROM debian:bookworm-slim

WORKDIR /usr/local/share/insta-game/seed

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    openarena-server \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /usr/local/share/insta-game/seed/baseoa

COPY --from=go-builder /insta-game /usr/local/bin/insta-game
COPY --from=openarena-builder /openarena-ded /usr/local/share/insta-game/seed/openarena-ded
COPY docker-containers/openarena/server.cfg /usr/local/share/insta-game/seed/default-server.cfg

EXPOSE 27960/udp
EXPOSE 5001/tcp

ENV DATA_SEED=/usr/local/share/insta-game/seed DATA_ROOT=/opt

WORKDIR /opt

ENTRYPOINT ["/usr/local/bin/insta-game", "agent"]
