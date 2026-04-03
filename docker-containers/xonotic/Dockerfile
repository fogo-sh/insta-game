FROM golang:1.22-bookworm AS sidecar-builder
WORKDIR /build
COPY sidecar/ .
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /sidecar .

FROM debian:bookworm-slim AS builder

RUN apt-get update && apt-get -y install --no-install-recommends \
    ca-certificates gcc git make \
    zlib1g-dev libjpeg-dev libpng-dev libcurl4-openssl-dev \
    automake autoconf libtool libgmp-dev \
    && rm -rf /var/lib/apt/lists/*

RUN git clone https://gitlab.com/xonotic/xonotic.git /build/xonotic
WORKDIR /build/xonotic
RUN ./all update -l best -r darkplaces d0_blind_id

WORKDIR /build/xonotic/d0_blind_id
RUN ./autogen.sh && ./configure && make && make install && ldconfig

WORKDIR /build/xonotic/darkplaces
RUN make sv-release

FROM debian:bookworm-slim

RUN apt-get update && apt-get -y install --no-install-recommends \
    libcurl4 libgmp10 libjpeg62-turbo libpng16-16 zlib1g \
    && rm -rf /var/lib/apt/lists/*

COPY --from=sidecar-builder /sidecar /usr/local/bin/sidecar

WORKDIR /opt

COPY --from=builder /build/xonotic/darkplaces/darkplaces-dedicated ./xonotic-linux-arm64-dedicated
COPY --from=builder /usr/local/lib/libd0_blind_id* /usr/local/lib/
RUN ldconfig

COPY Xonotic-clean/data ./data
RUN mkdir -p /opt/data && touch /opt/data/server.cfg

COPY server.cfg /opt/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
