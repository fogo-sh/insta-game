FROM --platform=$BUILDPLATFORM golang:1.27.3-bookworm AS go-builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
COPY internal/ ./internal/
COPY cmd/ ./cmd/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build GOMAXPROCS=2 GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -p 2 -o /insta-game ./cmd/insta-game

FROM debian:bookworm-slim AS runtime
ARG TARGETARCH
RUN test "$TARGETARCH" = amd64 \
    && dpkg --add-architecture i386 \
    && apt-get update && apt-get install -y --no-install-recommends \
       ca-certificates lib32gcc-s1 lib32stdc++6 libcurl4:i386 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 game

FROM runtime AS server
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*
USER game
WORKDIR /home/game
RUN mkdir steamcmd
WORKDIR /home/game/steamcmd
RUN curl -fsSL https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz | tar -xz
ARG STEAM_APP
ARG STEAM_MOD
# Fresh SteamCMD needs app metadata before app_update to avoid "Missing configuration".
RUN set --; \
    if [ "$STEAM_APP" = 90 ]; then set -- +app_set_config 90 mod "$STEAM_MOD"; fi; \
    ./steamcmd.sh +force_install_dir /home/game/server +login anonymous +app_info_update 1 +app_info_print "$STEAM_APP" "$@" +app_update "$STEAM_APP" validate +quit \
    && test -f /home/game/server/"$STEAM_MOD"/liblist.gam

FROM runtime
RUN mkdir -p /opt /home/game/.steam/sdk32 /usr/local/share/insta-game/seed \
    && chown game:game /opt
ARG GAME_ID
COPY --chmod=0644 docker-containers/${GAME_ID}/server.cfg /usr/local/share/insta-game/seed/default-server.cfg
COPY --from=server /home/game/server/ /usr/local/share/insta-game/seed/
COPY --from=server /home/game/steamcmd/linux32/steamclient.so /usr/local/share/insta-game/seed/linux32/steamclient.so
RUN ln -s /opt/linux32/steamclient.so /home/game/.steam/sdk32/steamclient.so
COPY --from=go-builder /insta-game /usr/local/bin/insta-game
ENV DATA_SEED=/usr/local/share/insta-game/seed DATA_ROOT=/opt LD_LIBRARY_PATH=/opt:/opt/linux32
USER game
WORKDIR /opt
ENTRYPOINT ["/usr/local/bin/insta-game", "agent"]
