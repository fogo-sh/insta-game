FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ ./
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim AS ioquake3-builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cmake \
    git \
    libcurl4-openssl-dev \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 https://github.com/ioquake/ioq3.git /src/ioq3

WORKDIR /src/ioq3

RUN cmake -S . -B build \
    -DBUILD_SERVER=ON \
    -DBUILD_CLIENT=OFF \
    -DBUILD_RENDERER_OPENGL1=OFF \
    -DBUILD_RENDERER_OPENGL2=OFF \
    -DBUILD_GAME_LIBRARIES=ON \
    -DBUILD_GAME_QVMS=OFF \
    -DCMAKE_BUILD_TYPE=Release \
    && cmake --build build --parallel \
    && cmake --install build --prefix /runtime

FROM debian:bookworm-slim

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libcurl4 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/baseq3 /opt/data

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar
COPY --from=ioquake3-builder /runtime/ /opt/

COPY docker-containers/ioquake3/game.json /opt/game.json
COPY docker-containers/ioquake3/server.cfg /opt/default-server.cfg

EXPOSE 27960/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
