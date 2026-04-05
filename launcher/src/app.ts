import { Hono } from "hono";
import { streamSSE } from "hono/streaming";
import type { Backend, GameConfig } from "./backend.js";
import type { GameCache } from "./cache.js";
import { makeDiscordHandler } from "./discord.js";
import { log } from "./logger.js";
import { renderUi, renderRowHeader, type GameUiConfig } from "./ui.js";

const WEB_UI_PASSPHRASE = process.env.WEB_UI_PASSPHRASE ?? "";
const API_TOKEN = process.env.API_TOKEN ?? "";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";
const PUBLIC_HOST = process.env.PUBLIC_HOST ?? "localhost";

function statusFragment(state: { status: string; publicIp?: string; players?: number }): string {
  const ip = state.publicIp ? ` — ${state.publicIp}` : "";
  const players = state.players ? ` (${state.players} players)` : "";
  return `<span class="status ${state.status}">${state.status}${ip}${players}</span>`;
}

function gameUiConfig(config: GameConfig): GameUiConfig {
  const connectPort = config.connectPort as number | undefined;
  const clientDownloadUrl = config.clientDownloadUrl as string | undefined;
  return {
    connectAddress: connectPort ? `${PUBLIC_HOST}:${connectPort}` : null,
    clientDownloadUrl: clientDownloadUrl ?? null,
  };
}

export function createApp(backend: Backend, cache: GameCache): Hono {
  const app = new Hono();

  // Public status page — no auth required
  app.get("/", async c => {
    const passphrase = c.req.header("x-passphrase") ?? "";
    const game = c.req.query("game");
    const operation = c.req.query("operation");

    if (game && operation) {
      if (passphrase !== WEB_UI_PASSPHRASE) {
        log.warn(`web: auth failure from ${c.req.header("x-forwarded-for") ?? "unknown"}`);
        return c.text("unauthorized", 401);
      }
      const games = backend.getGames();
      const config = games[game];
      if (!config) return c.html(`<span class="status">unknown game: ${game}</span>`, 400);
      let state;
      if (operation === "start") {
        log.info(`web: start ${game}`);
        state = await backend.startGame(config);
        log.info(`web: start ${game} → ${state.status}`);
        cache.set(game, await backend.getCachedState(config));
      } else if (operation === "stop") {
        log.info(`web: stop ${game}`);
        state = await backend.stopGame(config);
        log.info(`web: stop ${game} → ${state.status}`);
        cache.set(game, await backend.getCachedState(config));
      } else {
        state = await backend.getGameState(config);
      }
      return c.html(statusFragment(state));
    }

    // Passphrase validation ping (HX-Request with no game/operation)
    if (c.req.header("hx-request") && WEB_UI_PASSPHRASE !== "") {
      if (passphrase !== WEB_UI_PASSPHRASE) return c.text("unauthorized", 401);
      return c.text("ok");
    }

    // Render public page
    const games = backend.getGames();
    const rows = Object.entries(games).map(([key, config]) => ({
      key,
      state: cache.get(key) ?? { status: "offline" as const, players: 0, hostname: "", map: "", updatedAt: new Date() },
      ui: gameUiConfig(config),
    }));
    return c.html(renderUi(rows));
  });

  // Web UI POST — start/stop from htmx buttons (auth required)
  app.post("/", async c => {
    const passphrase = c.req.header("x-passphrase") ?? "";
    if (passphrase !== WEB_UI_PASSPHRASE) {
      log.warn(`web: auth failure from ${c.req.header("x-forwarded-for") ?? "unknown"}`);
      const isHtmx = !!c.req.header("hx-request");
      if (isHtmx) return c.html(`<span class="status">unauthorized</span>`, 401);
      return c.json({ error: "unauthorized" }, 401);
    }

    const game = c.req.query("game");
    const opFromQuery = c.req.query("operation");
    const isHtmx = !!c.req.header("hx-request");

    let gameKey: string;
    let operation: string;

    if (game && opFromQuery) {
      gameKey = game; operation = opFromQuery;
    } else {
      const body = await c.req.json<{ game: string; operation: string }>();
      gameKey = body.game; operation = body.operation;
    }

    const games = backend.getGames();
    const config = games[gameKey];
    if (!config) {
      if (isHtmx) return c.html(`<span class="status">unknown game: ${gameKey}</span>`, 400);
      return c.json({ error: `unknown game: ${gameKey}` }, 400);
    }

    let state;
    if (operation === "start") {
      log.info(`web: start ${gameKey}`);
      state = await backend.startGame(config);
      log.info(`web: start ${gameKey} → ${state.status}`);
      cache.set(gameKey, await backend.getCachedState(config));
    } else if (operation === "stop") {
      log.info(`web: stop ${gameKey}`);
      state = await backend.stopGame(config);
      log.info(`web: stop ${gameKey} → ${state.status}`);
      cache.set(gameKey, await backend.getCachedState(config));
    } else {
      state = await backend.getGameState(config);
    }

    if (isHtmx) return c.html(statusFragment(state));
    return c.json(state);
  });

  // Public fragment endpoint — returns updated row header for htmx 30s poll
  app.get("/status", c => {
    const game = c.req.query("game") ?? "";
    const games = backend.getGames();
    if (!games[game]) return c.text(`unknown game: ${game}`, 400);
    const state = cache.get(game) ?? { status: "offline" as const, players: 0, hostname: "", map: "", updatedAt: new Date() };
    return c.html(renderRowHeader(game, state));
  });

  // SSE log proxy — auth via token query param
  app.get("/logs", async c => {
    const token = c.req.query("token") ?? "";
    if (token !== WEB_UI_PASSPHRASE) return c.text("unauthorized", 401);

    const game = c.req.query("game") ?? "";
    const games = backend.getGames();
    const config = games[game];
    if (!config) return c.text(`unknown game: ${game}`, 400);

    const state = await backend.getGameState(config);
    if (state.status === "offline" || !state.publicIp) return c.text("game offline", 503);

    const sidecarUrl = `http://${state.publicIp}:${config.sidecarPort}/logs`;

    return streamSSE(c, async stream => {
      log.info(`logs: stream opened for ${game}`);
      await stream.writeSSE({ data: `[connecting to ${game} logs]`, event: "log" });
      let res: Response;
      try {
        res = await fetch(sidecarUrl, {
          headers: { Authorization: `Bearer ${SIDECAR_TOKEN}` },
          signal: c.req.raw.signal,
        });
      } catch (error) {
        log.info(`logs: stream closed for ${game} (connection error)`);
        await stream.writeSSE({ data: `[log proxy error: ${error instanceof Error ? error.message : String(error)}]`, event: "log" });
        await stream.close();
        return;
      }
      if (!res.ok || !res.body) {
        log.warn(`logs: sidecar returned ${res.status} for ${game}`);
        await stream.writeSSE({ data: `[log proxy error: sidecar returned HTTP ${res.status}]`, event: "log" });
        await stream.close();
        return;
      }
      await stream.writeSSE({ data: `[connected to ${game} logs]`, event: "log" });
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split("\n");
          buf = lines.pop() ?? "";
          for (const line of lines) {
            if (line.startsWith("data: ")) await stream.writeSSE({ data: line.slice(6), event: "log" });
          }
        }
      } catch { /* client disconnected */ }
      log.info(`logs: stream closed for ${game}`);
    });
  });

  // JSON API
  app.get("/api", async c => {
    const token = c.req.header("x-api-token") ?? "";
    if (token !== API_TOKEN) return c.json({ error: "unauthorized" }, 401);
    const game = c.req.query("game") ?? "";
    const operation = c.req.query("operation");
    const games = backend.getGames();
    const config = games[game];
    if (!config) return c.json({ error: `unknown game: ${game}` }, 400);
    if (operation === "start") {
      log.info(`api: start ${game}`);
      const state = await backend.startGame(config);
      log.info(`api: start ${game} → ${state.status}`);
      cache.set(game, await backend.getCachedState(config));
      return c.json(state);
    }
    if (operation === "stop") {
      log.info(`api: stop ${game}`);
      const state = await backend.stopGame(config);
      log.info(`api: stop ${game} → ${state.status}`);
      cache.set(game, await backend.getCachedState(config));
      return c.json(state);
    }
    return c.json(await backend.getGameState(config));
  });

  // Discord webhook
  app.post("/discord", makeDiscordHandler(backend));

  return app;
}
