FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS go-builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
COPY internal/ ./internal/
COPY cmd/ ./cmd/
RUN GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /insta-game ./cmd/insta-game

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

WORKDIR /usr/local/share/insta-game/seed

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /usr/local/share/insta-game/seed/id1 /usr/local/share/insta-game/seed/data

COPY --from=go-builder /insta-game /usr/local/bin/insta-game
COPY --from=fteqw-builder /usr/local/bin/fteqw.sv /usr/local/share/insta-game/seed/fteqw.sv
COPY --from=frikbot-downloader /opt/frikbot /usr/local/share/insta-game/seed/frikbot

COPY docker-containers/fteqw/server.cfg /usr/local/share/insta-game/seed/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENV DATA_SEED=/usr/local/share/insta-game/seed DATA_ROOT=/opt

WORKDIR /opt

ENTRYPOINT ["/usr/local/bin/insta-game", "agent"]
