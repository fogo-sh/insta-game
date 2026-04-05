FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    openarena-server \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/baseoa

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY docker-containers/openarena/game.json /opt/game.json
COPY docker-containers/openarena/server.cfg /opt/default-server.cfg

EXPOSE 27960/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
