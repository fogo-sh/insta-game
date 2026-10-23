FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS go-builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
COPY internal/ ./internal/
COPY cmd/ ./cmd/
RUN GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /insta-game ./cmd/insta-game

FROM debian:bookworm-slim

WORKDIR /usr/local/share/insta-game/seed

RUN apt-get update && apt-get install -y --no-install-recommends \
    bzflag-server \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /usr/local/share/insta-game/seed/data

COPY --from=go-builder /insta-game /usr/local/bin/insta-game
COPY docker-containers/bzflag/server.cfg /usr/local/share/insta-game/seed/default-server.cfg

EXPOSE 5154/tcp
EXPOSE 5154/udp
EXPOSE 5001/tcp

ENV DATA_SEED=/usr/local/share/insta-game/seed DATA_ROOT=/opt

WORKDIR /opt

ENTRYPOINT ["/usr/local/bin/insta-game", "agent"]
