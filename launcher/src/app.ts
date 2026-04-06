import { Hono } from "hono";
import { streamSSE } from "hono/streaming";
import { CloudWatchLogsClient, FilterLogEventsCommand } from "@aws-sdk/client-cloudwatch-logs";
import type { Backend, CachedGameState, GameConfig } from "./backend.js";
import type { GameCache } from "./cache.js";
import type { DockerGameConfig } from "./backends/docker.js";
import { makeDiscordHandler } from "./discord.js";
import { log } from "./logger.js";
import { renderUi, type GameUiConfig } from "./ui.js";

const WEB_UI_PASSPHRASE = process.env.WEB_UI_PASSPHRASE ?? "";
const API_TOKEN = process.env.API_TOKEN ?? "";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";
const PUBLIC_HOST = process.env.PUBLIC_HOST ?? "localhost";
const BACKEND = process.env.BACKEND ?? "ecs";
const ENABLE_LOG_STREAMS = (process.env.ENABLE_LOG_STREAMS ?? "1") === "1";
const REGION = process.env.AWS_REGION ?? "ca-central-1";
// SIDECAR_HOST is the internal address used to reach sidecars from the launcher process.
// For Docker deployments this differs from PUBLIC_HOST (host.docker.internal vs localhost).
const SIDECAR_HOST = process.env.SIDECAR_HOST ?? "localhost";
const cloudwatchLogs = new CloudWatchLogsClient({ region: REGION });

interface LauncherGameConfig extends GameConfig {
  serviceName?: string;
  logGroupName?: string;
}

function statusFragment(state: { status: string; publicIp?: string; players?: number }): string {
  const ip = state.publicIp ? ` — ${state.publicIp}` : "";
  const players = state.players ? ` (${state.players} players)` : "";
  return `<span class="status ${state.status}">${state.status}${ip}${players}</span>`;
}

function gameUiConfig(gameKey: string, config: GameConfig, state: CachedGameState, startBlocked: boolean): GameUiConfig {
  const c = config as DockerGameConfig;
  const logMode = BACKEND === "ecs" ? "poll" : "sse";
  return {
    displayName: c.displayName ?? null,
    connectAddress: c.connectPort ? `${PUBLIC_HOST}:${c.connectPort}` : null,
    clientDownloadUrl: c.clientDownloadUrl ?? null,
    startBlocked,
    logsEnabled: ENABLE_LOG_STREAMS,
    logMode,
    logUrl: `/logs?game=${encodeURIComponent(gameKey)}`,
  };
}

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

// Returns the set of host ports currently occupied by non-offline games
function occupiedHostPorts(
  games: Record<string, GameConfig>,
  cache: GameCache
): Set<string> {
  const occupied = new Set<string>();
  for (const [key, config] of Object.entries(games)) {
    const state = cache.get(key);
    if (!state || state.status === "offline") continue;
    const ports = (config as DockerGameConfig).ports ?? {};
    for (const binding of Object.values(ports)) {
      occupied.add(binding.hostPort);
    }
  }
  return occupied;
}

// Returns true if any of this game's host ports are occupied by another game
function hasPortConflict(
  config: GameConfig,
  occupied: Set<string>,
  ownKey: string,
  games: Record<string, GameConfig>,
  cache: GameCache
): boolean {
  const ownState = cache.get(ownKey);
  // Don't block a game that's already running/starting — it owns its ports
  if (ownState && ownState.status !== "offline") return false;
  const ports = (config as DockerGameConfig).ports ?? {};
  for (const binding of Object.values(ports)) {
    if (occupied.has(binding.hostPort)) return true;
  }
  return false;
}

export function createApp(backend: Backend, cache: GameCache): Hono {
  const app = new Hono();

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
    if (status >= 500) {
      log.error(base);
    } else if (status >= 400) {
      log.warn(base);
    } else {
      log.info(base);
    }
  });

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
    await cache.refreshIfStale();
    const games = backend.getGames();
    const occupied = occupiedHostPorts(games, cache);
    const rows = Object.entries(games).map(([key, config]) => {
      const state = cache.get(key) ?? { status: "offline" as const, players: 0, hostname: "", map: "", updatedAt: new Date() };
      return {
        key,
        state,
        ui: gameUiConfig(key, config, state, hasPortConflict(config, occupied, key, games, cache)),
      };
    });
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

  // Batched status endpoint — returns cached state for every game in one response.
  app.get("/status", async c => {
    await cache.refreshIfStale();
    const games = backend.getGames();
    const states = Object.fromEntries(
      Object.keys(games).map(game => [
        game,
        cache.get(game) ?? { status: "offline" as const, players: 0, hostname: "", map: "", updatedAt: new Date() },
      ])
    );
    return c.json(states);
  });

  // Logs endpoint:
  // - ECS: short-poll CloudWatch Logs to avoid long-lived Lambda invocations.
  // - Docker: SSE proxy to the local sidecar.
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
      return c.json({
        lines: splitLogMessages(res.events ?? []),
        cursor: res.nextToken ?? cursor ?? null,
      });
    }

    const cached = cache.get(game);
    if (!cached || cached.status === "offline") return c.text("game offline", 503);

    const sidecarUrl = `http://${SIDECAR_HOST}:${config.sidecarPort}/logs`;

    return streamSSE(c, async stream => {
      log.info(`logs: stream opened for ${game}`);
      await stream.writeSSE({ data: `<div class="log-line">[connecting to ${game} logs]</div>`, event: "log" });
      let res: Response;
      try {
        res = await fetch(sidecarUrl, {
          headers: { Authorization: `Bearer ${SIDECAR_TOKEN}` },
          signal: c.req.raw.signal,
        });
      } catch (error) {
        log.info(`logs: stream closed for ${game} (connection error)`);
        await stream.writeSSE({ data: `<div class="log-line">[log proxy error: ${error instanceof Error ? error.message : String(error)}]</div>`, event: "log" });
        await stream.close();
        return;
      }
      if (!res.ok || !res.body) {
        log.warn(`logs: sidecar returned ${res.status} for ${game}`);
        await stream.writeSSE({ data: `<div class="log-line">[log proxy error: sidecar returned HTTP ${res.status}]</div>`, event: "log" });
        await stream.close();
        return;
      }
      await stream.writeSSE({ data: `<div class="log-line">[connected to ${game} logs]</div>`, event: "log" });
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
            if (line.startsWith("data: ")) await stream.writeSSE({ data: `<div class="log-line">${line.slice(6)}</div>`, event: "log" });
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
