FROM golang:1.27.3-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -o /insta-game ./cmd/insta-game
COPY docker-containers/ /game-sources/
RUN mkdir /catalog && for game in /game-sources/*; do \
    name="$(basename "$game")"; mkdir "/catalog/$name"; \
    cp "$game/game.json" "/catalog/$name/"; \
    for config in server.cfg UnrealTournament.ini; do \
      if [ -f "$game/$config" ]; then cp "$game/$config" "/catalog/$name/"; fi; \
    done; \
    done

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /insta-game /usr/local/bin/insta-game
COPY --from=build /catalog/ /opt/games/
ENTRYPOINT ["insta-game"]
CMD ["serve", "--listen", "0.0.0.0:3000", "--data", "/data", "--catalog", "/opt/games"]
