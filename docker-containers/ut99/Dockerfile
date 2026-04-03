FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS sidecar-builder
ARG TARGETARCH
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libstdc++6 \
    libsdl2-2.0-0 \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/data

ENV DEFAULT_CONFIG_PATH=/usr/local/share/ut99/UnrealTournament.ini

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY docker-containers/ut99/UnrealTournament.ini /usr/local/share/ut99/UnrealTournament.ini
COPY --chmod=755 docker-containers/ut99/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY --chmod=755 docker-containers/ut99/start-ut99.sh /usr/local/bin/start-ut99.sh

EXPOSE 7777/udp
EXPOSE 7778/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
