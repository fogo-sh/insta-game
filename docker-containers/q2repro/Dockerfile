FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim AS q2repro-builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    build-essential \
    git \
    meson \
    ninja-build \
    pkg-config \
    libsdl2-dev \
    libcurl4-openssl-dev \
    libjpeg-dev \
    libopenal-dev \
    libpng-dev \
    libvorbis-dev \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --recurse-submodules https://github.com/Paril/q2repro.git /src/q2repro

WORKDIR /src/q2repro

RUN meson setup builddir \
    && meson compile -C builddir \
    && install -m 0755 "$(find builddir -type f \( -name q2reproded -o -name q2proded -o -name q2pro \) -perm /111 | head -n 1)" /usr/local/bin/q2proded \
    && mkdir -p /usr/local/share/baseq2 \
    && find builddir -type f -name 'game*.so' -exec install -m 0644 {} /usr/local/share/baseq2/ \;

FROM debian:bookworm-slim

COPY --from=q2repro-builder /usr/local/bin/q2proded /opt/q2proded
COPY --from=q2repro-builder /usr/local/share/baseq2/ /opt/baseq2/

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libcurl4 \
    libjpeg62-turbo \
    libopenal1 \
    libsdl2-2.0-0 \
    libvorbisfile3 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/baseq2 /opt/data \
    && useradd -r -s /bin/false game \
    && chown -R game:game /opt

USER game

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar

COPY docker-containers/q2repro/server.cfg /opt/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
