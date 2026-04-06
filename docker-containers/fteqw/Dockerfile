FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim AS omicron-downloader
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates wget unzip \
    && rm -rf /var/lib/apt/lists/*
RUN wget -O /tmp/obots.zip \
    "https://s3.amazonaws.com/neogeographica-downloads/mods/obots/obots102-fixed.zip" \
    && unzip /tmp/obots.zip -d /opt \
    && rm /tmp/obots.zip

FROM debian:bookworm-slim AS fteqw-builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    build-essential \
    git \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 https://github.com/fte-team/fteqw.git /src/fteqw

WORKDIR /src/fteqw/engine
RUN make sv-rel -j"$(nproc)" \
    && install -m 0755 release/fteqw-sv /usr/local/bin/fteqw.sv

FROM debian:bookworm-slim

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/id1 /opt/data

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY --from=fteqw-builder /usr/local/bin/fteqw.sv /opt/fteqw.sv
COPY --from=omicron-downloader /opt/omicron /opt/omicron

COPY docker-containers/fteqw/game.json /opt/game.json
COPY docker-containers/fteqw/server.cfg /opt/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
