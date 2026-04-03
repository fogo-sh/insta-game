import { serve } from "@hono/node-server";
import { createBackend } from "./backends/index.js";
import { createApp } from "./app.js";

const PORT = parseInt(process.env.PORT ?? "3000", 10);

const backend = createBackend();
const app = createApp(backend);

serve({ fetch: app.fetch, port: PORT }, info => {
  console.log(`launcher listening on http://localhost:${info.port}`);
});
