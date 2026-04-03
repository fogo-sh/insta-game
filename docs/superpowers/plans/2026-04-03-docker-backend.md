# Docker Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the launcher so backend-specific code (ECS/EC2 vs Docker) is isolated behind a shared interface, add a Docker backend, move the launcher out of `lambda/`, and wire up a docker-compose service for self-hosted use.

**Architecture:** Extract a `Backend` TypeScript interface with `getGames`, `getGameState`, `startGame`, `stopGame`. The existing ECS logic moves to `backends/ecs.ts`. A new `backends/docker.ts` implements the same interface using the Docker Engine Unix socket API. `index.ts` selects the backend via a `BACKEND` env var. The Docker backend reaches game sidecars on `localhost` using the host port mappings from Docker inspect.

**Tech Stack:** TypeScript, Hono, esbuild, Docker Engine HTTP API (Unix socket via Node `http` with `socketPath`), existing `@aws-sdk/*` (ECS backend only).

---

### Task 1: Move launcher out of `lambda/` and update references

**Files:**
- Move: `lambda/launcher/` → `launcher/`
- Modify: `pulumi/__main__.py` (update `FileArchive` path)
- Modify: `AGENTS.md` (update path references)
- Modify: `README.md` (update path references)

- [ ] **Step 1: Move the directory**

```bash
git mv lambda/launcher launcher
```

- [ ] **Step 2: Update Pulumi FileArchive path**

In `pulumi/__main__.py`, find:
```python
code=pulumi.FileArchive("../lambda/launcher/dist"),
```
Change to:
```python
code=pulumi.FileArchive("../launcher/dist"),
```

- [ ] **Step 3: Update AGENTS.md**

In `AGENTS.md`, find the section that references `lambda/launcher/` and update all occurrences to `launcher/`. The build commands block should read:

```sh
From `launcher/`:

npm install
npm run build       # bundle src/index.ts into dist/index.js
npm run register    # register Discord slash commands
```

- [ ] **Step 4: Update README.md**

Search for `lambda/launcher` in `README.md` and replace with `launcher`. Check that the `npm run register` example command path is correct.

- [ ] **Step 5: Verify build still works**

```bash
cd launcher
npm run build
```
Expected output: `dist/index.js  ~126kb`

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: move launcher out of lambda/ to top-level"
```

---

### Task 2: Define the Backend interface and split out ECS backend

**Files:**
- Create: `launcher/src/backend.ts`
- Create: `launcher/src/backends/ecs.ts`
- Modify: `launcher/src/index.ts` (import from backend, not games.ts directly)
- Modify: `launcher/src/discord.ts` (same)
- Keep: `launcher/src/games.ts` (delete at end of task)

- [ ] **Step 1: Create `launcher/src/backend.ts`**

```typescript
export interface GameConfig {
  sidecarPort: number;
  // Backend-specific fields (e.g. serviceName, containerName) are opaque to callers.
  [key: string]: unknown;
}

export interface GameState {
  status: "offline" | "starting" | "online";
  publicIp?: string;
  players: number;
  ready: boolean;
  configUrl?: string;
}

export interface Backend {
  getGames(): Record<string, GameConfig>;
  getGameState(config: GameConfig): Promise<GameState>;
  startGame(config: GameConfig, configUrl?: string): Promise<GameState>;
  stopGame(config: GameConfig): Promise<GameState>;
}
```

- [ ] **Step 2: Create `launcher/src/backends/ecs.ts`**

Copy the full contents of `launcher/src/games.ts` into this file, then make two changes:

1. Replace the top-level import line and exports so it exports a class implementing `Backend`:

```typescript
import { ECSClient, UpdateServiceCommand, ListTasksCommand, DescribeTasksCommand } from "@aws-sdk/client-ecs";
import { EC2Client, DescribeNetworkInterfacesCommand } from "@aws-sdk/client-ec2";
import type { Backend, GameConfig, GameState } from "../backend.js";

const REGION = process.env.AWS_REGION ?? "ca-central-1";
const CLUSTER = process.env.ECS_CLUSTER ?? "";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";
const MAX_POLLS = 10;
const POLL_INTERVAL_MS = 5000;

const ecs = new ECSClient({ region: REGION });
const ec2 = new EC2Client({ region: REGION });

// ECS game config — serviceName is the ECS service name
export interface EcsGameConfig extends GameConfig {
  serviceName: string;
}

export class EcsBackend implements Backend {
  getGames(): Record<string, EcsGameConfig> {
    return JSON.parse(process.env.GAMES ?? "{}");
  }

  async getGameState(config: GameConfig): Promise<GameState> {
    const c = config as EcsGameConfig;
    try {
      const listRes = await ecs.send(new ListTasksCommand({ cluster: CLUSTER, serviceName: c.serviceName }));
      const taskArn = listRes.taskArns?.[0];
      if (!taskArn) return { status: "offline", players: 0, ready: false };

      const descRes = await ecs.send(new DescribeTasksCommand({ cluster: CLUSTER, tasks: [taskArn] }));
      const task = descRes.tasks?.[0];
      const eniId = task?.attachments?.[0]?.details?.find(d => d.name === "networkInterfaceId")?.value;
      if (!eniId) return { status: "starting", players: 0, ready: false };

      const eniRes = await ec2.send(new DescribeNetworkInterfacesCommand({ NetworkInterfaceIds: [eniId] }));
      const publicIp = eniRes.NetworkInterfaces?.[0]?.Association?.PublicIp;
      if (!publicIp) return { status: "starting", players: 0, ready: false };

      const sidecar = await getSidecarStatus(publicIp, c.sidecarPort);
      if (!sidecar) return { status: "starting", publicIp, players: 0, ready: false };

      const running = Boolean(sidecar.running);
      const ready = Boolean(sidecar.ready);
      const players = Number(sidecar.players ?? 0);
      return { status: running && ready ? "online" : "starting", publicIp, players, ready };
    } catch {
      return { status: "offline", players: 0, ready: false };
    }
  }

  async stopGame(config: GameConfig): Promise<GameState> {
    const c = config as EcsGameConfig;
    await setDesiredCount(c.serviceName, 0);
    return waitForState(this, config, "offline");
  }

  async startGame(config: GameConfig, configUrl?: string): Promise<GameState> {
    const c = config as EcsGameConfig;
    await this.stopGame(config);
    await setDesiredCount(c.serviceName, 1);
    let state = await waitForState(this, config, "online");
    if (configUrl && state.status === "online" && state.publicIp) {
      await restartWithConfig(state.publicIp, c.sidecarPort, configUrl);
      state = await waitForState(this, config, "online");
      state.configUrl = configUrl;
    }
    return state;
  }
}

async function setDesiredCount(serviceName: string, count: number): Promise<void> {
  await ecs.send(new UpdateServiceCommand({ cluster: CLUSTER, service: serviceName, desiredCount: count }));
}

async function getSidecarStatus(ip: string, port: number): Promise<Record<string, unknown> | null> {
  try {
    const res = await fetch(`http://${ip}:${port}/status`, { signal: AbortSignal.timeout(5000) });
    if (!res.ok) return null;
    return res.json() as Promise<Record<string, unknown>>;
  } catch {
    return null;
  }
}

async function waitForState(backend: EcsBackend, config: GameConfig, desired: "online" | "offline"): Promise<GameState> {
  let state = await backend.getGameState(config);
  for (let i = 0; i < MAX_POLLS; i++) {
    if (state.status === desired) return state;
    await new Promise(r => setTimeout(r, POLL_INTERVAL_MS));
    state = await backend.getGameState(config);
  }
  return state;
}

async function restartWithConfig(ip: string, port: number, configUrl: string): Promise<void> {
  await fetch(`http://${ip}:${port}/restart`, {
    method: "POST",
    headers: { "Authorization": `Bearer ${SIDECAR_TOKEN}`, "Content-Type": "application/json" },
    body: JSON.stringify({ config_url: configUrl }),
    signal: AbortSignal.timeout(10000),
  });
}
```

- [ ] **Step 3: Create `launcher/src/backends/docker.ts`** (stub — full implementation in Task 3)

```typescript
import type { Backend, GameConfig, GameState } from "../backend.js";

export interface DockerGameConfig extends GameConfig {
  containerName: string;
}

export class DockerBackend implements Backend {
  getGames(): Record<string, DockerGameConfig> {
    return JSON.parse(process.env.GAMES ?? "{}");
  }

  async getGameState(_config: GameConfig): Promise<GameState> {
    return { status: "offline", players: 0, ready: false };
  }

  async startGame(_config: GameConfig): Promise<GameState> {
    return { status: "offline", players: 0, ready: false };
  }

  async stopGame(_config: GameConfig): Promise<GameState> {
    return { status: "offline", players: 0, ready: false };
  }
}
```

- [ ] **Step 4: Create `launcher/src/backends/index.ts`** — backend factory

```typescript
import type { Backend } from "../backend.js";
import { EcsBackend } from "./ecs.js";
import { DockerBackend } from "./docker.js";

export function createBackend(): Backend {
  const backend = process.env.BACKEND ?? "ecs";
  if (backend === "docker") return new DockerBackend();
  return new EcsBackend();
}
```

- [ ] **Step 5: Update `launcher/src/index.ts`**

Replace:
```typescript
import { getGames, getGameState, startGame, stopGame } from "./games.js";
```
With:
```typescript
import { createBackend } from "./backends/index.js";

const backend = createBackend();
const { getGames, getGameState, startGame, stopGame } = {
  getGames: () => backend.getGames(),
  getGameState: (c: Parameters<typeof backend.getGameState>[0]) => backend.getGameState(c),
  startGame: (c: Parameters<typeof backend.startGame>[0]) => backend.startGame(c),
  stopGame: (c: Parameters<typeof backend.stopGame>[0]) => backend.stopGame(c),
};
```

Actually cleaner to just use `backend` directly throughout `index.ts`. Replace all four call sites:
- `getGames()` → `backend.getGames()`
- `getGameState(config)` → `backend.getGameState(config)`
- `startGame(config)` → `backend.startGame(config)`
- `stopGame(config)` → `backend.stopGame(config)`

And remove the old import line.

- [ ] **Step 6: Update `launcher/src/discord.ts`**

Replace:
```typescript
import { getGames, getGameState, startGame, stopGame } from "./games.js";
```
With:
```typescript
import { createBackend } from "./backends/index.js";

const backend = createBackend();
```

Then replace all four usages in the file:
- `getGames()` → `backend.getGames()`
- `getGameState(config)` → `backend.getGameState(config)`
- `startGame(config)` → `backend.startGame(config)`
- `stopGame(config)` → `backend.stopGame(config)`

- [ ] **Step 7: Delete `launcher/src/games.ts`**

```bash
git rm launcher/src/games.ts
```

- [ ] **Step 8: Type-check and build**

```bash
cd launcher
npx tsc --noEmit
npm run build
```
Expected: 0 errors, `dist/index.js` built.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "refactor: extract Backend interface, move ECS logic to backends/ecs.ts"
```

---

### Task 3: Implement the Docker backend

**Files:**
- Modify: `launcher/src/backends/docker.ts`

The Docker backend communicates with the Docker Engine via its HTTP API over the Unix socket at `/var/run/docker.sock`. Node's built-in `http` module supports Unix sockets via the `socketPath` option.

The backend needs to:
1. **`getGameState`**: inspect the container → get host-port mapping for the sidecar port → call `http://localhost:<hostPort>/status`
2. **`startGame`**: start the container if stopped, wait for sidecar ready
3. **`stopGame`**: stop the container, wait for it to exit

- [ ] **Step 1: Write `launcher/src/backends/docker.ts`** (full implementation)

```typescript
import http from "http";
import type { Backend, GameConfig, GameState } from "../backend.js";

const SOCKET = process.env.DOCKER_SOCKET ?? "/var/run/docker.sock";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";
const MAX_POLLS = 20;
const POLL_INTERVAL_MS = 3000;

export interface DockerGameConfig extends GameConfig {
  containerName: string;
}

// Low-level Docker API call over Unix socket
function dockerRequest(method: string, path: string, body?: unknown): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const bodyStr = body ? JSON.stringify(body) : undefined;
    const req = http.request({
      socketPath: SOCKET,
      path,
      method,
      headers: {
        "Content-Type": "application/json",
        ...(bodyStr ? { "Content-Length": Buffer.byteLength(bodyStr) } : {}),
      },
    }, res => {
      const chunks: Buffer[] = [];
      res.on("data", c => chunks.push(c));
      res.on("end", () => {
        const text = Buffer.concat(chunks).toString();
        try { resolve(text ? JSON.parse(text) : null); }
        catch { resolve(text); }
      });
    });
    req.on("error", reject);
    if (bodyStr) req.write(bodyStr);
    req.end();
  });
}

// Inspect a container — returns Docker inspect object or null if not found
async function inspectContainer(name: string): Promise<Record<string, unknown> | null> {
  try {
    const data = await dockerRequest("GET", `/containers/${encodeURIComponent(name)}/json`);
    return data as Record<string, unknown>;
  } catch {
    return null;
  }
}

// Get the host port bound to a given container port (e.g. "5001/tcp")
function getHostPort(inspect: Record<string, unknown>, containerPort: number): number | null {
  const ports = (inspect.NetworkSettings as Record<string, unknown>)?.Ports as Record<string, Array<{ HostPort: string }>> | undefined;
  const key = `${containerPort}/tcp`;
  const binding = ports?.[key]?.[0];
  return binding ? parseInt(binding.HostPort, 10) : null;
}

async function getSidecarStatus(port: number): Promise<Record<string, unknown> | null> {
  try {
    const res = await fetch(`http://localhost:${port}/status`, { signal: AbortSignal.timeout(5000) });
    if (!res.ok) return null;
    return res.json() as Promise<Record<string, unknown>>;
  } catch {
    return null;
  }
}

async function waitForState(backend: DockerBackend, config: GameConfig, desired: "online" | "offline"): Promise<GameState> {
  let state = await backend.getGameState(config);
  for (let i = 0; i < MAX_POLLS; i++) {
    if (state.status === desired) return state;
    await new Promise(r => setTimeout(r, POLL_INTERVAL_MS));
    state = await backend.getGameState(config);
  }
  return state;
}

export class DockerBackend implements Backend {
  getGames(): Record<string, DockerGameConfig> {
    return JSON.parse(process.env.GAMES ?? "{}");
  }

  async getGameState(config: GameConfig): Promise<GameState> {
    const c = config as DockerGameConfig;
    try {
      const inspect = await inspectContainer(c.containerName);
      if (!inspect) return { status: "offline", players: 0, ready: false };

      const state = inspect.State as Record<string, unknown>;
      if (!state?.Running) return { status: "offline", players: 0, ready: false };

      const hostPort = getHostPort(inspect, c.sidecarPort);
      if (!hostPort) return { status: "starting", players: 0, ready: false };

      const sidecar = await getSidecarStatus(hostPort);
      if (!sidecar) return { status: "starting", players: 0, ready: false };

      const running = Boolean(sidecar.running);
      const ready = Boolean(sidecar.ready);
      const players = Number(sidecar.players ?? 0);
      // For Docker, publicIp is the host machine's IP — callers use localhost
      return { status: running && ready ? "online" : "starting", publicIp: "localhost", players, ready };
    } catch {
      return { status: "offline", players: 0, ready: false };
    }
  }

  async startGame(config: GameConfig): Promise<GameState> {
    const c = config as DockerGameConfig;
    await dockerRequest("POST", `/containers/${encodeURIComponent(c.containerName)}/start`);
    return waitForState(this, config, "online");
  }

  async stopGame(config: GameConfig): Promise<GameState> {
    const c = config as DockerGameConfig;
    // t=15 gives the container 15s to stop gracefully before SIGKILL
    await dockerRequest("POST", `/containers/${encodeURIComponent(c.containerName)}/stop?t=15`);
    return waitForState(this, config, "offline");
  }
}
```

- [ ] **Step 2: Type-check and build**

```bash
cd launcher
npx tsc --noEmit
npm run build
```
Expected: 0 errors, bundle builds.

- [ ] **Step 3: Commit**

```bash
git add launcher/src/backends/docker.ts
git commit -m "feat(docker-backend): implement Docker Engine API backend"
```

---

### Task 4: Add launcher service to compose.yml

**Files:**
- Modify: `compose.yml`
- Modify: `launcher/package.json` (add `start` script for non-Lambda use)

The launcher runs as a plain Node process when not on Lambda. Hono's `handle(app)` wraps for Lambda, but `index.ts` also needs to be runnable as a plain HTTP server. Add a second entry point for this.

- [ ] **Step 1: Create `launcher/src/server.ts`** — plain Node HTTP entry point

```typescript
import { serve } from "@hono/node-server";
import { Hono } from "hono";
import { createBackend } from "./backends/index.js";
import { discordHandler } from "./discord.js";
import { renderUi } from "./ui.js";
import { streamSSE } from "hono/streaming";

// Re-assemble the app (same routes as index.ts, without Lambda adapter)
// Note: keep in sync with index.ts — consider extracting shared app setup
// into app.ts if they diverge significantly.

const WEB_UI_PASSPHRASE = process.env.WEB_UI_PASSPHRASE ?? "";
const API_TOKEN = process.env.API_TOKEN ?? "";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";
const PORT = parseInt(process.env.PORT ?? "3000", 10);

const backend = createBackend();
const app = new Hono();

function statusFragment(state: { status: string; publicIp?: string; players?: number }): string {
  const ip = state.publicIp ? ` — ${state.publicIp}` : "";
  const players = state.players ? ` (${state.players} players)` : "";
  return `<span class="status ${state.status}">${state.status}${ip}${players}</span>`;
}

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
  if (operation === "start") state = await backend.startGame(config);
  else if (operation === "stop") state = await backend.stopGame(config);
  else state = await backend.getGameState(config);
  if (isHtmx) return c.html(statusFragment(state));
  return c.json(state);
});

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
    let res: Response;
    try {
      res = await fetch(sidecarUrl, { headers: { Authorization: `Bearer ${SIDECAR_TOKEN}` }, signal: c.req.raw.signal });
    } catch { await stream.close(); return; }
    if (!res.ok || !res.body) { await stream.close(); return; }
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
  });
});

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

app.post("/discord", discordHandler);

serve({ fetch: app.fetch, port: PORT }, info => {
  console.log(`launcher listening on http://localhost:${info.port}`);
});
```

- [ ] **Step 2: Add `@hono/node-server` dependency**

```bash
cd launcher
npm install @hono/node-server
```

- [ ] **Step 3: Update `launcher/package.json` build scripts**

Add a `build:docker` script that bundles `server.ts` instead of `index.ts`, and a `start` script:

```json
"scripts": {
  "build": "esbuild src/index.ts --bundle --platform=node --target=node22 --outfile=dist/index.js --external:@aws-sdk/*",
  "build:watch": "esbuild src/index.ts --bundle --platform=node --target=node22 --outfile=dist/index.js --external:@aws-sdk/* --watch",
  "build:docker": "esbuild src/server.ts --bundle --platform=node --target=node22 --outfile=dist/server.js",
  "register": "tsx register-commands.ts",
  "start": "node dist/server.js"
}
```

- [ ] **Step 4: Add `launcher` service to `compose.yml`**

Append to `compose.yml`:

```yaml
  launcher:
    image: node:22-alpine
    working_dir: /app
    command: ["node", "dist/server.js"]
    volumes:
      - ./launcher/dist:/app/dist:ro
      - /var/run/docker.sock:/var/run/docker.sock
    ports:
      - "127.0.0.1:3000:3000/tcp"
    environment:
      BACKEND: "docker"
      PORT: "3000"
      WEB_UI_PASSPHRASE: "${WEB_UI_PASSPHRASE}"
      API_TOKEN: "${API_TOKEN}"
      SIDECAR_TOKEN: "${SIDECAR_TOKEN:-abc123}"
      GAMES: |
        {
          "xonotic":  {"containerName": "insta-game-xonotic-1",  "sidecarPort": 5001},
          "qssm":     {"containerName": "insta-game-qssm-1",     "sidecarPort": 5001},
          "q2repro":  {"containerName": "insta-game-q2repro-1",  "sidecarPort": 5001},
          "bzflag":   {"containerName": "insta-game-bzflag-1",   "sidecarPort": 5001}
        }
```

> **Note:** Docker Compose container names default to `<project>-<service>-<index>`. When running from the repo root as `docker compose up`, the project name defaults to the directory name (`insta-game`), giving names like `insta-game-xonotic-1`. Adjust if you use a different project name.

- [ ] **Step 5: Build the docker bundle and verify it starts**

```bash
cd launcher
npm run build:docker
```
Expected: `dist/server.js` created.

Then test it starts (will fail to connect to Docker if not running, but should print the listen message):
```bash
BACKEND=docker WEB_UI_PASSPHRASE=test API_TOKEN=test SIDECAR_TOKEN=test \
  GAMES='{"xonotic":{"containerName":"x","sidecarPort":5001}}' \
  node dist/server.js
```
Expected: `launcher listening on http://localhost:3000`

Ctrl+C to stop.

- [ ] **Step 6: Type-check**

```bash
cd launcher
npx tsc --noEmit
```
Expected: 0 errors.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: add Docker backend and compose launcher service"
```

---

### Task 5: Update AGENTS.md and README

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`

- [ ] **Step 1: Update AGENTS.md build commands**

The `lambda/launcher/` section should now read `launcher/` and include both build targets:

```markdown
From `launcher/`:

\`\`\`sh
npm install
npm run build          # Lambda bundle (dist/index.js)
npm run build:docker   # Docker bundle (dist/server.js)
npm run register       # register Discord slash commands
\`\`\`
```

- [ ] **Step 2: Update README self-hosted section**

Add a section explaining the Docker backend. After the existing Lambda/AWS deployment section, add:

```markdown
## Self-hosted (Docker)

The launcher can run locally against Docker instead of ECS.

1. Build the Docker bundle:
   ```sh
   cd launcher
   npm run build:docker
   ```

2. Set environment variables in `.env`:
   ```
   WEB_UI_PASSPHRASE=your-passphrase
   API_TOKEN=your-api-token
   SIDECAR_TOKEN=abc123
   ```

3. Start everything:
   ```sh
   docker compose up launcher
   ```

The launcher will be available at `http://localhost:3000`. It manages the other game containers via the Docker socket.
```

- [ ] **Step 3: Commit**

```bash
git add AGENTS.md README.md
git commit -m "docs: update for Docker backend and launcher restructure"
```
