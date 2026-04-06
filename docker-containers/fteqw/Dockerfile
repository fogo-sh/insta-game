FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

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

FROM debian:bookworm-slim AS frikbot-downloader
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates wget unzip \
    && rm -rf /var/lib/apt/lists/*

# FrikBot X++ — includes waypoints for DM1-6 and QC source.
# Uses standard QCC field names (no anonymous fields), so its prebuilt
# progs.dat is compatible with FTE without recompilation.
RUN wget -O /tmp/fbxpp.zip \
    "https://s3.amazonaws.com/neogeographica-downloads/mods/fbxpp/fbx%2B%2B.zip" \
    && mkdir -p /opt/frikbot \
    && unzip /tmp/fbxpp.zip -d /opt/frikbot \
    && rm -rf /tmp/fbxpp.zip /opt/frikbot/qc_src

FROM debian:bookworm-slim

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/id1 /opt/data

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY --from=fteqw-builder /usr/local/bin/fteqw.sv /opt/fteqw.sv
COPY --from=frikbot-downloader /opt/frikbot /opt/frikbot

COPY docker-containers/fteqw/game.json /opt/game.json
COPY docker-containers/fteqw/server.cfg /opt/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
