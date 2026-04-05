FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    bzflag-server \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/data

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY docker-containers/bzflag/game.json /opt/game.json
COPY docker-containers/bzflag/server.cfg /opt/default-server.cfg

EXPOSE 5154/tcp
EXPOSE 5154/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
