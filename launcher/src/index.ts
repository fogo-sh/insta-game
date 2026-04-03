import { handle } from "hono/aws-lambda";
import { createBackend } from "./backends/index.js";
import { createApp } from "./app.js";

const backend = createBackend();
const app = createApp(backend);

export const handler = handle(app);
