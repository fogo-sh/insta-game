import { readFileSync } from "fs";
import { join } from "path";
import { Hono } from "hono";
import { streamSSE } from "hono/streaming";
import { CloudWatchLogsClient, FilterLogEventsCommand } from "@aws-sdk/client-cloudwatch-logs";
import type { Backend, CachedGameState, GameConfig } from "./backend.js";
import type { GameCache } from "./cache.js";
import type { DockerGameConfig } from "./backends/docker.js";
import { makeDiscordHandler } from "./discord.js";
import { log } from "./logger.js";

const WEB_UI_PASSPHRASE = process.env.WEB_UI_PASSPHRASE ?? "";
const API_TOKEN = process.env.API_TOKEN ?? "";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";
const PUBLIC_HOST = process.env.PUBLIC_HOST ?? "localhost";
const BACKEND = process.env.BACKEND ?? "ecs";
const ENABLE_LOG_STREAMS = (process.env.ENABLE_LOG_STREAMS ?? "1") === "1";
const REGION = process.env.AWS_REGION ?? "ca-central-1";
const SIDECAR_HOST = process.env.SIDECAR_HOST ?? "localhost";
const cloudwatchLogs = new CloudWatchLogsClient({ region: REGION });

interface LauncherGameConfig extends GameConfig {
  serviceName?: string;
  logGroupName?: string;
}

const HTML_SHELL = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>insta-game</title>
  <link rel="stylesheet" href="/client.css">
</head>
<body>
  <div id="app"></div>
  <script src="/client.js"></script>
</body>
</html>`;

function splitLogMessages(events: Array<{ message?: string }>): string[] {
  const lines: string[] = [];
  for (const event of events) {
    const message = event.message ?? "";
    for (const line of message.split("\n")) {
      if (line !== "") lines.push(line);
    }
  }
  return lines;
}

function occupiedHostPorts(games: Record<string, GameConfig>, cache: GameCache): Set<string> {
  const occupied = new Set<string>();
  for (const [key, config] of Object.entries(games)) {
    const state = cache.get(key);
    if (!state || state.status === "offline") continue;
    const ports = (config as DockerGameConfig).ports ?? {};
    for (const binding of Object.values(ports)) occupied.add(binding.hostPort);
  }
  return occupied;
}

function hasPortConflict(
  config: GameConfig,
  occupied: Set<string>,
  ownKey: string,
  games: Record<string, GameConfig>,
  cache: GameCache
): boolean {
  const ownState = cache.get(ownKey);
  if (ownState && ownState.status !== "offline") return false;
  const ports = (config as DockerGameConfig).ports ?? {};
  for (const binding of Object.values(ports)) {
    if (occupied.has(binding.hostPort)) return true;
  }
  return false;
}

function buildGameEntry(
  key: string,
  config: GameConfig,
  state: CachedGameState,
  startBlocked: boolean
) {
  const c = config as DockerGameConfig;
  return {
    ...state,
    displayName: c.displayName ?? key,
    connectAddress: c.connectPort ? `${PUBLIC_HOST}:${c.connectPort}` : null,
    clientDownloadUrl: c.clientDownloadUrl ?? null,
    startBlocked,
  };
}

export function createApp(backend: Backend, cache: GameCache): Hono {
  const app = new Hono();

  // Load client bundle once at startup
  let clientBundle: Buffer | null = null;
  let clientCss: Buffer | null = null;
  try {
    clientBundle = readFileSync(join(process.cwd(), "dist/client.js"));
    clientCss = readFileSync(join(process.cwd(), "dist/client.css"));
  } catch {
    log.warn("app: dist/client.js not found — run npm run build:client");
  }

  app.use("*", async (c, next) => {
    const startedAt = Date.now();
    const method = c.req.method;
    const path = c.req.path;
    const query = c.req.query();
    const game = query.game ? ` game=${query.game}` : "";
    const operation = query.operation ? ` op=${query.operation}` : "";
    try {
      await next();
    } catch (error) {
      log.error(`http: ${method} ${path}${game}${operation} failed`, error);
      throw error;
    }
    const durationMs = Date.now() - startedAt;
    const status = c.res.status;
    const base = `http: ${method} ${path}${game}${operation} -> ${status} (${durationMs}ms)`;
    if (status >= 500) log.error(base);
    else if (status >= 400) log.warn(base);
    else log.info(base);
  });

  // Serve compiled client bundle
  app.get("/client.js", c => {
    if (!clientBundle) return c.text("client bundle not found — run npm run build:client", 503);
    return new Response(new Uint8Array(clientBundle), {
      headers: { "Content-Type": "application/javascript" },
    });
  });

  // Serve compiled client CSS
  app.get("/client.css", c => {
    if (!clientCss) return c.text("client CSS not found — run npm run build:client", 503);
    return new Response(new Uint8Array(clientCss), {
      headers: { "Content-Type": "text/css" },
    });
  });

  // HTML shell + passphrase validation ping
  app.get("/", async c => {
    const passphrase = c.req.header("x-passphrase") ?? "";

    // Passphrase validation ping from Preact client
    if (c.req.header("x-validate") && WEB_UI_PASSPHRASE !== "") {
      if (passphrase !== WEB_UI_PASSPHRASE) return c.text("unauthorized", 401);
      return c.text("ok");
    }

    // Legacy action dispatch (game + operation query params)
    const game = c.req.query("game");
    const operation = c.req.query("operation");
    if (game && operation) {
      if (passphrase !== WEB_UI_PASSPHRASE) {
        log.warn(`web: auth failure from ${c.req.header("x-forwarded-for") ?? "unknown"}`);
        return c.json({ error: "unauthorized" }, 401);
      }
      const games = backend.getGames();
      const config = games[game];
      if (!config) return c.json({ error: `unknown game: ${game}` }, 400);
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
      return c.json(state);
    }

    return c.html(HTML_SHELL);
  });

  // Start/stop actions from Preact client
  app.post("/", async c => {
    const passphrase = c.req.header("x-passphrase") ?? "";
    if (passphrase !== WEB_UI_PASSPHRASE) {
      log.warn(`web: auth failure from ${c.req.header("x-forwarded-for") ?? "unknown"}`);
      return c.json({ error: "unauthorized" }, 401);
    }

    const game = c.req.query("game");
    const opFromQuery = c.req.query("operation");
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
    if (!config) return c.json({ error: `unknown game: ${gameKey}` }, 400);

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
    return c.json(state);
  });

  // Batched status — returns state + metadata for all games
  app.get("/status", async c => {
    await cache.refreshIfStale();
    const games = backend.getGames();
    const occupied = occupiedHostPorts(games, cache);
    const result = Object.fromEntries(
      Object.entries(games).map(([key, config]) => {
        const state = cache.get(key) ?? { status: "offline" as const, players: 0, hostname: "", map: "", updatedAt: new Date() };
        return [key, buildGameEntry(key, config, state, hasPortConflict(config, occupied, key, games, cache))];
      })
    );
    return c.json(result);
  });

  // Logs endpoint
  app.get("/logs", async c => {
    if (!ENABLE_LOG_STREAMS) return c.text("log streaming disabled for this deployment", 503);

    const token = c.req.query("token") ?? "";
    if (token !== WEB_UI_PASSPHRASE) return c.text("unauthorized", 401);

    const game = c.req.query("game") ?? "";
    const games = backend.getGames();
    const config = games[game];
    if (!config) return c.text(`unknown game: ${game}`, 400);

    if (BACKEND === "ecs") {
      const ecsConfig = config as LauncherGameConfig;
      if (!ecsConfig.logGroupName) return c.json({ error: "log group not configured" }, 503);
      const cursor = c.req.query("cursor") ?? undefined;
      const res = await cloudwatchLogs.send(new FilterLogEventsCommand({
        logGroupName: ecsConfig.logGroupName,
        nextToken: cursor,
        limit: 100,
        startTime: cursor ? undefined : Date.now() - 5 * 60 * 1000,
      }));
      c.header("X-Log-Mode", "poll");
      return c.json({
        lines: splitLogMessages(res.events ?? []),
        cursor: res.nextToken ?? cursor ?? null,
      });
    }

    const cached = cache.get(game);
    if (!cached || cached.status === "offline") return c.text("game offline", 503);

    const sidecarUrl = `http://${SIDECAR_HOST}:${config.sidecarPort}/logs`;
    c.header("X-Log-Mode", "sse");

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
            if (line.startsWith("data: ")) {
              await stream.writeSSE({ data: line.slice(6), event: "log" });
            }
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

  app.post("/discord", makeDiscordHandler(backend));

  return app;
}
