FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

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

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    openarena-server \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/baseoa

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY --from=openarena-builder /openarena-ded /opt/openarena-ded
COPY docker-containers/openarena/game.json /opt/game.json
COPY docker-containers/openarena/server.cfg /opt/default-server.cfg

EXPOSE 27960/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
