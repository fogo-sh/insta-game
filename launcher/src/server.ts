import { serve } from "@hono/node-server";
import { createBackend } from "./backends/index.js";
import { GameCache } from "./cache.js";
import { createApp } from "./app.js";
import { log } from "./logger.js";

const PORT = parseInt(process.env.PORT ?? "3000", 10);
const BACKEND = process.env.BACKEND ?? "ecs";

const backend = createBackend();
const cache = new GameCache(backend);
cache.start();
const app = createApp(backend, cache);

serve({ fetch: app.fetch, port: PORT }, info => {
  log.info(`launcher listening on http://localhost:${info.port} (backend: ${BACKEND})`);
  const games = Object.keys(backend.getGames());
  log.info(`games: ${games.length > 0 ? games.join(", ") : "(none configured)"}`);
});
