import { Hono } from "hono";
import { handle } from "hono/aws-lambda";
import { getGames, getGameState, startGame, stopGame } from "./games.js";
import { discordHandler } from "./discord.js";
import { renderUi } from "./ui.js";

const WEB_UI_PASSPHRASE = process.env.WEB_UI_PASSPHRASE ?? "";
const API_TOKEN = process.env.API_TOKEN ?? "";

const app = new Hono();

// Web UI
app.get("/", c => c.html(renderUi()));

// Web UI form submission
app.post("/", async c => {
  const passphrase = c.req.header("x-passphrase") ?? "";
  if (passphrase !== WEB_UI_PASSPHRASE) return c.json({ error: "unauthorized" }, 401);

  const body = await c.req.json<{ game: string; operation: string }>();
  const games = getGames();
  const config = games[body.game];
  if (!config) return c.json({ error: `unknown game: ${body.game}` }, 400);

  if (body.operation === "start") {
    const state = await startGame(config);
    return c.json(state);
  }
  if (body.operation === "stop") {
    const state = await stopGame(config);
    return c.json(state);
  }
  const state = await getGameState(config);
  return c.json(state);
});

// JSON API
app.get("/api", async c => {
  const token = c.req.header("x-api-token") ?? "";
  if (token !== API_TOKEN) return c.json({ error: "unauthorized" }, 401);

  const game = c.req.query("game") ?? "";
  const operation = c.req.query("operation");
  const games = getGames();
  const config = games[game];
  if (!config) return c.json({ error: `unknown game: ${game}` }, 400);

  if (operation === "start") return c.json(await startGame(config));
  if (operation === "stop") return c.json(await stopGame(config));
  return c.json(await getGameState(config));
});

// Discord webhook
app.post("/discord", discordHandler);

export const handler = handle(app);
