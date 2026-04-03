import { Hono } from "hono";
import { streamSSE } from "hono/streaming";
import type { Backend } from "./backend.js";
import { makeDiscordHandler } from "./discord.js";
import { renderUi } from "./ui.js";

const WEB_UI_PASSPHRASE = process.env.WEB_UI_PASSPHRASE ?? "";
const API_TOKEN = process.env.API_TOKEN ?? "";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";

function statusFragment(state: { status: string; publicIp?: string; players?: number }): string {
  const ip = state.publicIp ? ` — ${state.publicIp}` : "";
  const players = state.players ? ` (${state.players} players)` : "";
  return `<span class="status ${state.status}">${state.status}${ip}${players}</span>`;
}

export function createApp(backend: Backend): Hono {
  const app = new Hono();

  // Web UI — full page or htmx fragment
  app.get("/", async c => {
    const game = c.req.query("game");
    const operation = c.req.query("operation");

    if (game && operation) {
      const passphrase = c.req.header("x-passphrase") ?? "";
      if (passphrase !== WEB_UI_PASSPHRASE) return c.text("unauthorized", 401);

      const games = backend.getGames();
      const config = games[game];
      if (!config) return c.html(`<span class="status">unknown game: ${game}</span>`, 400);

      let state;
      if (operation === "start") state = await backend.startGame(config);
      else if (operation === "stop") state = await backend.stopGame(config);
      else state = await backend.getGameState(config);

      return c.html(statusFragment(state));
    }

    const games = backend.getGames();
    return c.html(renderUi(Object.keys(games)));
  });

  // Web UI form submission / htmx button posts
  app.post("/", async c => {
    const passphrase = c.req.header("x-passphrase") ?? "";
    if (passphrase !== WEB_UI_PASSPHRASE) {
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
      gameKey = game;
      operation = opFromQuery;
    } else {
      const body = await c.req.json<{ game: string; operation: string }>();
      gameKey = body.game;
      operation = body.operation;
    }

    const games = backend.getGames();
    const config = games[gameKey];
    if (!config) {
      if (isHtmx) return c.html(`<span class="status">unknown game: ${gameKey}</span>`, 400);
      return c.json({ error: `unknown game: ${gameKey}` }, 400);
    }

    let state;
    if (operation === "start") state = await backend.startGame(config);
    else if (operation === "stop") state = await backend.stopGame(config);
    else state = await backend.getGameState(config);

    if (isHtmx) return c.html(statusFragment(state));
    return c.json(state);
  });

  // SSE log proxy — auth via token query param (EventSource can't send headers)
  app.get("/logs", async c => {
    const token = c.req.query("token") ?? "";
    if (token !== WEB_UI_PASSPHRASE) return c.text("unauthorized", 401);

    const game = c.req.query("game") ?? "";
    const games = backend.getGames();
    const config = games[game];
    if (!config) return c.text(`unknown game: ${game}`, 400);

    const state = await backend.getGameState(config);
    if (state.status === "offline" || !state.publicIp) {
      return c.text("game offline", 503);
    }

    const sidecarUrl = `http://${state.publicIp}:${config.sidecarPort}/logs`;

    return streamSSE(c, async stream => {
      let res: Response;
      try {
        res = await fetch(sidecarUrl, {
          headers: { Authorization: `Bearer ${SIDECAR_TOKEN}` },
          signal: c.req.raw.signal,
        });
      } catch {
        await stream.close();
        return;
      }

      if (!res.ok || !res.body) {
        await stream.close();
        return;
      }

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
      } catch {
        // Client disconnected or sidecar closed — clean exit
      }
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

    if (operation === "start") return c.json(await backend.startGame(config));
    if (operation === "stop") return c.json(await backend.stopGame(config));
    return c.json(await backend.getGameState(config));
  });

  // Discord webhook
  app.post("/discord", makeDiscordHandler(backend));

  return app;
}
