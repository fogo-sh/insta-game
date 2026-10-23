# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS go-builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
COPY internal/ ./internal/
COPY cmd/ ./cmd/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /insta-game ./cmd/insta-game

FROM debian:bookworm-slim AS builder

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get -y install --no-install-recommends \
    ca-certificates gcc git make \
    zlib1g-dev libjpeg-dev libpng-dev libcurl4-openssl-dev \
    automake autoconf libtool libgmp-dev

RUN git clone --depth 1 --filter=blob:none https://gitlab.com/xonotic/xonotic.git /build/xonotic
WORKDIR /build/xonotic
RUN ./all update -l best -r darkplaces d0_blind_id

WORKDIR /build/xonotic/d0_blind_id
RUN ./autogen.sh && ./configure && make && make install && ldconfig

WORKDIR /build/xonotic/darkplaces
RUN make sv-release

FROM debian:bookworm-slim

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get -y install --no-install-recommends \
    ca-certificates libcurl4 libgmp10 libjpeg62-turbo libpng16-16 zlib1g

COPY --from=go-builder /insta-game /usr/local/bin/insta-game

WORKDIR /usr/local/share/insta-game/seed

COPY --from=builder /build/xonotic/darkplaces/darkplaces-dedicated ./xonotic-linux-arm64-dedicated
COPY --from=builder /usr/local/lib/libd0_blind_id* /usr/local/lib/
RUN ldconfig

COPY docker-containers/xonotic/Xonotic-clean/data ./data
RUN mkdir -p /usr/local/share/insta-game/seed/data && touch /usr/local/share/insta-game/seed/data/server.cfg

COPY docker-containers/xonotic/server.cfg /usr/local/share/insta-game/seed/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENV DATA_SEED=/usr/local/share/insta-game/seed DATA_ROOT=/opt

WORKDIR /opt

ENTRYPOINT ["/usr/local/bin/insta-game", "agent"]
