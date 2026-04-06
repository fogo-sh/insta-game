import { streamHandle } from "hono/aws-lambda";
import { createBackend } from "./backends/index.js";
import { GameCache } from "./cache.js";
import { createApp } from "./app.js";

const backend = createBackend();
const cache = new GameCache(backend);
const app = createApp(backend, cache);

export const handler = streamHandle(app);
