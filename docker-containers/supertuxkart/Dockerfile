FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim

ARG STK_VERSION=1.5
ARG STK_ARCH=arm64

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    libbluetooth3 \
    libcurl4 \
    libgl1 \
    libjpeg62-turbo \
    libopenal1 \
    libpng16-16 \
    libsqlite3-0 \
    libstdc++6 \
    libvorbisfile3 \
    libxrandr2 \
    libxxf86vm1 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://github.com/supertuxkart/stk-code/releases/download/${STK_VERSION}/SuperTuxKart-${STK_VERSION}-linux-${STK_ARCH}.tar.gz" \
    | tar -xz -C /opt \
    && mv "/opt/SuperTuxKart-${STK_VERSION}-linux-${STK_ARCH}" /opt/stk \
    && mkdir -p /opt/data

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY docker-containers/supertuxkart/game.json /opt/game.json
COPY docker-containers/supertuxkart/server_config.xml /opt/default-server.cfg

EXPOSE 2757/udp
EXPOSE 2759/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
