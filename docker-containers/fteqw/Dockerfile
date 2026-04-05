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
    && install -m 0755 "$(find release/sv_linux -maxdepth 1 -type f -perm /111 | head -n 1)" /usr/local/bin/fteqw.sv

FROM debian:bookworm-slim

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/id1 /opt/data

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY --from=fteqw-builder /usr/local/bin/fteqw.sv /opt/fteqw.sv

COPY docker-containers/fteqw/server.cfg /opt/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
