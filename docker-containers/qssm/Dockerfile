FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim AS qssm-builder

ARG QSSM_VERSION=1.6.5

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    build-essential \
    pkg-config \
    libcurl4-openssl-dev \
    libgl1-mesa-dev \
    libgnutls28-dev \
    libmad0-dev \
    libopus-dev \
    libopusfile-dev \
    libsdl2-dev \
    libvorbis-dev \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

RUN curl -L \
    "https://github.com/timbergeron/QSS-M/archive/refs/tags/${QSSM_VERSION}.tar.gz" \
    -o /tmp/qssm.tar.gz \
    && tar -xzf /tmp/qssm.tar.gz --strip-components=1 -C /src \
    && make -C Quake USE_SDL2=1 -j"$(nproc)" \
    && install -m 0755 Quake/quakespasm /usr/local/bin/qssm \
    && install -m 0644 Quake/quakespasm.pak /usr/local/share/quakespasm.pak \
    && install -m 0644 Quake/qssm.pak /usr/local/share/qssm.pak

FROM debian:bookworm-slim

COPY --from=qssm-builder /usr/local/bin/qssm /opt/qssm
COPY --from=qssm-builder /usr/local/share/quakespasm.pak /opt/quakespasm.pak
COPY --from=qssm-builder /usr/local/share/qssm.pak /opt/qssm.pak

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    libcurl4 \
    libgl1 \
    libgnutls30 \
    libmad0 \
    libopusfile0 \
    libsdl2-2.0-0 \
    libvorbisfile3 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/id1 /opt/data

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar

COPY docker-containers/qssm/server.cfg /opt/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
