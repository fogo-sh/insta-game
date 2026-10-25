FROM --platform=$BUILDPLATFORM golang:1.27.3-bookworm AS go-builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
COPY internal/ ./internal/
COPY cmd/ ./cmd/
RUN GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /insta-game ./cmd/insta-game

FROM debian:bookworm-slim

WORKDIR /usr/local/share/insta-game/seed

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libstdc++6 \
    libsdl2-2.0-0 \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /usr/local/share/insta-game/seed/data

ENV DEFAULT_CONFIG_PATH=/usr/local/share/ut99/UnrealTournament.ini

COPY --from=go-builder /insta-game /usr/local/bin/insta-game
COPY docker-containers/ut99/UnrealTournament.ini /usr/local/share/ut99/UnrealTournament.ini
COPY --chmod=755 docker-containers/ut99/start-ut99.sh /usr/local/bin/start-ut99.sh

EXPOSE 7777/udp
EXPOSE 7778/udp
EXPOSE 7779/udp
EXPOSE 7780/udp
EXPOSE 7781/udp
EXPOSE 5001/tcp

ENV DATA_SEED=/usr/local/share/insta-game/seed DATA_ROOT=/opt

WORKDIR /opt

ENTRYPOINT ["/usr/local/bin/insta-game", "agent"]
