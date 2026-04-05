FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim AS smokinguns-builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    git \
    libcurl4-openssl-dev \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 --branch ioq3-synchro https://github.com/smokin-guns/SmokinGuns.git /src/SmokinGuns

WORKDIR /src/SmokinGuns

RUN make -j"$(nproc)" \
    BUILD_SERVER=1 \
    BUILD_CLIENT=0 \
    BUILD_GAME_SO=0 \
    BUILD_GAME_QVM=0 \
    BUILD_STANDALONE=1 \
    BASEGAME=smokinguns \
    SERVERBIN=smokingunsded \
    && install -m 0755 build/release-linux-aarch64/smokingunsded.aarch64 /smokingunsded

FROM debian:bookworm-slim

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libcurl4 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/smokinguns

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY --from=smokinguns-builder /smokingunsded /opt/smokingunsded
COPY docker-containers/smokinguns/game.json /opt/game.json
COPY docker-containers/smokinguns/server.cfg /opt/default-server.cfg

EXPOSE 27960/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
