# Public Status Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign `GET /` so unauthenticated visitors see live game server status in full-width accordion rows, while authenticated admins get start/stop/logs controls inline in the same page.

**Architecture:** A `GameCache` class polls all sidecars every 30s and stores `{ status, players, hostname, map }` per game. `GET /` renders always-public accordion rows from the cache. A new `GET /status?game=` endpoint returns htmx fragments for 30s client-side refresh. Admin controls (start/stop/logs) reveal inline per-accordion after passphrase validation. A `PUBLIC_HOST` env var controls the connect address shown to players. `connectPort` and `clientDownloadUrl` are new optional fields in the `GAMES` JSON config.

**Tech Stack:** TypeScript, Hono, htmx, esbuild, Node `http` (existing Docker socket pattern).

---

### Task 1: Add `CachedGameState` to backend interface and implement in both backends

**Files:**
- Modify: `launcher/src/backend.ts`
- Modify: `launcher/src/backends/docker.ts`
- Modify: `launcher/src/backends/ecs.ts`

- [ ] **Step 1: Add `CachedGameState` type and `getCachedState` to `backend.ts`**

Replace the full contents of `launcher/src/backend.ts` with:

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

// Extended state used by the public cache — includes sidecar-reported fields
export interface CachedGameState {
  status: "offline" | "starting" | "online";
  players: number;
  hostname: string;
  map: string;
  updatedAt: Date;
}

export interface Backend {
  getGames(): Record<string, GameConfig>;
  getGameState(config: GameConfig): Promise<GameState>;
  getCachedState(config: GameConfig): Promise<CachedGameState>;
  startGame(config: GameConfig, configUrl?: string): Promise<GameState>;
  stopGame(config: GameConfig): Promise<GameState>;
}
```

- [ ] **Step 2: Implement `getCachedState` in `launcher/src/backends/docker.ts`**

The Docker backend already has `inspectContainer` and `getSidecarStatus`. Add `getCachedState` to the `DockerBackend` class, after `getGameState`:

```typescript
async getCachedState(config: GameConfig): Promise<CachedGameState> {
  const c = config as DockerGameConfig;
  const offline: CachedGameState = { status: "offline", players: 0, hostname: "", map: "", updatedAt: new Date() };
  try {
    const inspect = await inspectContainer(c.containerName);
    if (!inspect) return offline;
    const state = inspect.State as Record<string, unknown>;
    if (!state?.Running) return offline;
    const hostPort = getHostPort(inspect, c.sidecarPort);
    if (!hostPort) return { ...offline, status: "starting" };
    const sidecar = await getSidecarStatus(hostPort);
    if (!sidecar) return { ...offline, status: "starting" };
    const running = Boolean(sidecar.running);
    const ready = Boolean(sidecar.ready);
    return {
      status: running && ready ? "online" : "starting",
      players: Number(sidecar.players ?? 0),
      hostname: String(sidecar.hostname ?? ""),
      map: String(sidecar.map ?? ""),
      updatedAt: new Date(),
    };
  } catch {
    return offline;
  }
}
```

Also add `CachedGameState` to the import at the top of `docker.ts`:

```typescript
import type { Backend, GameConfig, GameState, CachedGameState } from "../backend.js";
```

- [ ] **Step 3: Implement `getCachedState` in `launcher/src/backends/ecs.ts`**

The ECS backend reaches sidecars via the task's public IP. Add `getCachedState` to the `EcsBackend` class, after `getGameState`:

```typescript
async getCachedState(config: GameConfig): Promise<CachedGameState> {
  const c = config as EcsGameConfig;
  const offline: CachedGameState = { status: "offline", players: 0, hostname: "", map: "", updatedAt: new Date() };
  try {
    const listRes = await ecs.send(new ListTasksCommand({ cluster: CLUSTER, serviceName: c.serviceName }));
    const taskArn = listRes.taskArns?.[0];
    if (!taskArn) return offline;
    const descRes = await ecs.send(new DescribeTasksCommand({ cluster: CLUSTER, tasks: [taskArn] }));
    const task = descRes.tasks?.[0];
    const eniId = task?.attachments?.[0]?.details?.find(d => d.name === "networkInterfaceId")?.value;
    if (!eniId) return { ...offline, status: "starting" };
    const eniRes = await ec2.send(new DescribeNetworkInterfacesCommand({ NetworkInterfaceIds: [eniId] }));
    const publicIp = eniRes.NetworkInterfaces?.[0]?.Association?.PublicIp;
    if (!publicIp) return { ...offline, status: "starting" };
    const sidecar = await getSidecarStatus(publicIp, c.sidecarPort);
    if (!sidecar) return { ...offline, status: "starting" };
    const running = Boolean(sidecar.running);
    const ready = Boolean(sidecar.ready);
    return {
      status: running && ready ? "online" : "starting",
      players: Number(sidecar.players ?? 0),
      hostname: String(sidecar.hostname ?? ""),
      map: String(sidecar.map ?? ""),
      updatedAt: new Date(),
    };
  } catch {
    return offline;
  }
}
```

Also add `CachedGameState` to the import at the top of `ecs.ts`:

```typescript
import type { Backend, GameConfig, GameState, CachedGameState } from "../backend.js";
```

- [ ] **Step 4: Type-check**

```bash
cd launcher && npx tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 5: Commit**

```bash
git add launcher/src/backend.ts launcher/src/backends/docker.ts launcher/src/backends/ecs.ts
git commit -m "feat: add CachedGameState and getCachedState to Backend interface"
```

---

### Task 2: Create `GameCache` — background poller

**Files:**
- Create: `launcher/src/cache.ts`

- [ ] **Step 1: Write `launcher/src/cache.ts`**

```typescript
import type { Backend, CachedGameState } from "./backend.js";
import { log } from "./logger.js";

const POLL_INTERVAL_MS = 30_000;

export class GameCache {
  private cache = new Map<string, CachedGameState>();
  private timer: ReturnType<typeof setInterval> | null = null;

  constructor(private readonly backend: Backend) {}

  start(): void {
    // Poll immediately on startup, then every 30s
    void this.pollAll();
    this.timer = setInterval(() => { void this.pollAll(); }, POLL_INTERVAL_MS);
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer);
  }

  get(gameKey: string): CachedGameState | null {
    return this.cache.get(gameKey) ?? null;
  }

  set(gameKey: string, state: CachedGameState): void {
    this.cache.set(gameKey, state);
  }

  private async pollAll(): Promise<void> {
    const games = this.backend.getGames();
    await Promise.all(
      Object.entries(games).map(async ([key, config]) => {
        try {
          const state = await this.backend.getCachedState(config);
          this.cache.set(key, state);
        } catch (err) {
          log.error(`cache: poll failed for ${key}`, err);
          this.cache.set(key, {
            status: "offline",
            players: 0,
            hostname: "",
            map: "",
            updatedAt: new Date(),
          });
        }
      })
    );
  }
}
```

- [ ] **Step 2: Type-check**

```bash
cd launcher && npx tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add launcher/src/cache.ts
git commit -m "feat: add GameCache background poller"
```

---

### Task 3: Rewrite the UI — accordion rows

**Files:**
- Modify: `launcher/src/ui.tsx`

This is a full rewrite of `ui.tsx`. The existing grid-of-cards approach is replaced with full-width accordion rows. The passphrase auth moves from a top-level gate to a per-row inline prompt inside each accordion's expanded section.

- [ ] **Step 1: Rewrite `launcher/src/ui.tsx`**

```typescript
/** @jsxImportSource hono/jsx */
import type { FC } from "hono/jsx";
import type { CachedGameState } from "./backend.js";

const SESSION_KEY = "insta-game-passphrase";

const css = `
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { background: #111; color: #eee; font-family: monospace; padding: 2rem; }
  h1 { margin-bottom: 2rem; font-size: 1.4rem; }

  .accordion { display: flex; flex-direction: column; gap: 0.5rem; }

  .row { border: 1px solid #333; background: #1a1a1a; }
  .row-header {
    display: flex; align-items: center; gap: 1rem;
    padding: 0.75rem 1rem; cursor: pointer; user-select: none;
    width: 100%;
  }
  .row-header:hover { background: #222; }
  .status-dot { font-size: 0.8rem; flex-shrink: 0; }
  .game-name { font-weight: bold; min-width: 8rem; }
  .row-meta { display: flex; gap: 1.5rem; flex: 1; color: #aaa; font-size: 0.85rem; flex-wrap: wrap; }
  .row-meta .online { color: #4f4; }
  .row-meta .starting { color: #fa4; }
  .row-meta .offline { color: #666; }
  .expand-btn {
    background: none; border: none; color: #aaa; cursor: pointer;
    font-family: monospace; font-size: 0.85rem; padding: 0; flex-shrink: 0;
  }

  .row-body { border-top: 1px solid #333; padding: 1rem; display: none; }
  .row-body.open { display: block; }

  .row-details { display: flex; gap: 2rem; align-items: flex-start; flex-wrap: wrap; margin-bottom: 1rem; }
  .connect code { background: #222; padding: 0.2rem 0.5rem; border: 1px solid #444; cursor: pointer; }
  .connect code:hover { background: #2a2a2a; }
  .client-link { font-size: 0.85rem; color: #aaa; }
  .client-link a { color: #88f; text-decoration: none; }
  .client-link a:hover { text-decoration: underline; }

  .admin-section { margin-top: 0.75rem; border-top: 1px solid #222; padding-top: 0.75rem; }
  .admin-auth input {
    padding: 0.4rem; background: #222; color: #eee;
    border: 1px solid #444; font-family: monospace; margin-right: 0.5rem;
  }
  .admin-auth button { padding: 0.4rem 0.8rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; }
  .admin-controls { display: flex; gap: 0.5rem; flex-wrap: wrap; }
  .admin-controls button { padding: 0.4rem 0.8rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; }
  .admin-controls button:hover { background: #444; }

  .status-frag { font-size: 0.85rem; color: #aaa; margin-top: 0.5rem; }
  .status-frag .online { color: #4f4; }
  .status-frag .starting { color: #fa4; }

  dialog { background: #111; color: #eee; border: 1px solid #444; padding: 0; width: calc(100vw - 4rem); max-width: 960px; height: calc(100vh - 4rem); display: flex; flex-direction: column; }
  dialog::backdrop { background: rgba(0,0,0,0.7); }
  .dialog-header { display: flex; align-items: center; justify-content: space-between; padding: 0.75rem 1rem; border-bottom: 1px solid #333; font-size: 0.85rem; }
  .dialog-header span { color: #aaa; }
  .dialog-close { background: none; border: none; color: #aaa; cursor: pointer; font-size: 1.2rem; font-family: monospace; padding: 0 0.25rem; }
  .dialog-close:hover { color: #eee; }
  .log-panel { flex: 1; overflow-y: scroll; background: #0a0a0a; font-size: 0.75rem; padding: 0.75rem; }
  .log-line { white-space: pre-wrap; word-break: break-all; line-height: 1.5; }
`;

const initScript = `
(function() {
  var SESSION_KEY = ${JSON.stringify(SESSION_KEY)};

  // Retrieve stored passphrase
  function getPassphrase() {
    return sessionStorage.getItem(SESSION_KEY) || "";
  }

  // Toggle accordion open/close
  window.toggleRow = function(game) {
    var body = document.getElementById("row-body-" + game);
    var btn = document.getElementById("expand-btn-" + game);
    var open = body.classList.toggle("open");
    btn.textContent = open ? "[collapse ▲]" : "[expand ▼]";
  };

  // Copy connect address to clipboard
  window.copyConnect = function(text) {
    navigator.clipboard.writeText(text).catch(function() {});
  };

  // Admin unlock — validate passphrase then swap auth form for controls
  window.adminUnlock = function(game) {
    var input = document.getElementById("admin-input-" + game);
    var btn = document.getElementById("admin-unlock-btn-" + game);
    var val = input.value;
    if (!val) return;
    btn.disabled = true;
    btn.textContent = "checking...";
    fetch("/", { headers: { "X-Passphrase": val, "HX-Request": "true" } })
      .then(function(res) {
        if (res.status === 401) {
          btn.disabled = false;
          btn.textContent = "unlock";
          input.style.borderColor = "#f44";
          return;
        }
        sessionStorage.setItem(SESSION_KEY, val);
        showAdminControls(game, val);
      })
      .catch(function() {
        btn.disabled = false;
        btn.textContent = "unlock";
      });
  };

  function showAdminControls(game, pp) {
    var section = document.getElementById("admin-section-" + game);
    section.innerHTML = adminControlsHtml(game);
    htmx.process(section);
    // Set passphrase header on the controls
    section.setAttribute("hx-headers", JSON.stringify({"X-Passphrase": pp}));
  }

  function adminControlsHtml(game) {
    return '<div class="admin-controls" hx-headers=\'{"X-Passphrase": ""}\' id="admin-controls-' + game + '">' +
      '<button hx-post="/?game=' + game + '&operation=start" hx-target="#status-result-' + game + '">start</button>' +
      '<button hx-post="/?game=' + game + '&operation=stop" hx-target="#status-result-' + game + '">stop</button>' +
      '<button onclick="toggleLogs(\\'' + game + '\\')" >logs</button>' +
      '</div>' +
      '<div id="status-result-' + game + '" class="status-frag"></div>';
  }

  // Re-apply passphrase header to all controls when already authenticated
  function restoreAdminControls() {
    var pp = getPassphrase();
    if (!pp) return;
    // Validate first
    fetch("/", { headers: { "X-Passphrase": pp, "HX-Request": "true" } })
      .then(function(res) {
        if (res.status === 401) { sessionStorage.removeItem(SESSION_KEY); return; }
        // Show controls for every game
        document.querySelectorAll("[data-admin-section]").forEach(function(el) {
          var game = el.getAttribute("data-admin-section");
          showAdminControls(game, pp);
        });
      });
  }

  window.toggleLogs = function(game) {
    var dialog = document.getElementById("log-dialog-" + game);
    if (dialog.open) { dialog.close(); return; }
    var pp = getPassphrase();
    var inner = document.getElementById("log-sse-" + game);
    if (!inner.getAttribute("sse-connect")) {
      inner.setAttribute("hx-ext", "sse");
      inner.setAttribute("sse-connect", "/logs?game=" + game + "&token=" + encodeURIComponent(pp));
      htmx.process(inner);
      var lines = document.getElementById("log-lines-" + game);
      var observer = new MutationObserver(function() { lines.scrollTop = lines.scrollHeight; });
      observer.observe(lines, { childList: true });
    }
    dialog.showModal();
  };

  restoreAdminControls();
})();
`;

interface StatusDotProps { status: string }
const StatusDot: FC<StatusDotProps> = ({ status }) => {
  if (status === "online") return <span class="status-dot">🟢</span>;
  if (status === "starting") return <span class="status-dot">🟡</span>;
  return <span class="status-dot">⚫</span>;
};

interface RowProps {
  game: string;
  state: CachedGameState;
  connectAddress: string | null;
  clientDownloadUrl: string | null;
}

const AccordionRow: FC<RowProps> = ({ game, state, connectAddress, clientDownloadUrl }) => {
  const metaOnline = state.status === "online";
  return (
    <div class="row" id={`row-${game}`}>
      {/* Collapsed header — always visible, auto-refreshed by htmx every 30s */}
      <div
        id={`row-header-${game}`}
        class="row-header"
        hx-get={`/status?game=${game}`}
        hx-trigger="every 30s"
        hx-target={`#row-header-${game}`}
        hx-swap="outerHTML"
        onclick={`toggleRow('${game}')`}
      >
        <StatusDot status={state.status} />
        <span class="game-name">{game}</span>
        <span class="row-meta">
          {metaOnline && state.hostname ? <span>{state.hostname}</span> : null}
          {metaOnline && state.map ? <span>{state.map}</span> : null}
          {metaOnline ? <span>{state.players} player{state.players !== 1 ? "s" : ""}</span> : null}
          {!metaOnline ? <span class={state.status}>{state.status}</span> : null}
        </span>
        <button class="expand-btn" id={`expand-btn-${game}`}>[expand ▼]</button>
      </div>

      {/* Expanded body */}
      <div class="row-body" id={`row-body-${game}`}>
        <div class="row-details">
          {connectAddress ? (
            <div class="connect">
              connect: <code onclick={`copyConnect('${connectAddress}')`} title="click to copy">{connectAddress}</code>
            </div>
          ) : null}
          {clientDownloadUrl ? (
            <div class="client-link">
              <a href={clientDownloadUrl} target="_blank" rel="noopener">get client ↗</a>
            </div>
          ) : null}
        </div>

        {/* Admin section — shows auth prompt until unlocked */}
        <div class="admin-section" id={`admin-section-${game}`} data-admin-section={game}>
          <div class="admin-auth">
            <input
              type="text"
              id={`admin-input-${game}`}
              placeholder="passphrase"
              autocomplete="off"
              spellcheck={false}
              style="letter-spacing:0.15em;"
              oninput={`this.style.borderColor=''`}
              onkeydown={`if(event.key==='Enter')adminUnlock('${game}')`}
            />
            <button id={`admin-unlock-btn-${game}`} onclick={`adminUnlock('${game}')`}>unlock</button>
          </div>
        </div>
      </div>

      {/* Log dialog */}
      <dialog id={`log-dialog-${game}`}>
        <div class="dialog-header">
          <span>{game} — logs</span>
          <button class="dialog-close" onclick={`document.getElementById('log-dialog-${game}').close()`}>✕</button>
        </div>
        <div id={`log-sse-${game}`}>
          <div id={`log-lines-${game}`} class="log-panel" sse-swap="log" hx-swap="beforeend" />
        </div>
      </dialog>
    </div>
  );
};

export interface GameUiConfig {
  connectAddress: string | null;
  clientDownloadUrl: string | null;
}

export function renderUi(
  games: Array<{ key: string; state: CachedGameState; ui: GameUiConfig }>
): string {
  const page = (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>insta-game</title>
        <style>{css}</style>
      </head>
      <body>
        <h1>insta-game</h1>
        <div class="accordion">
          {games.map(({ key, state, ui }) => (
            <AccordionRow
              key={key}
              game={key}
              state={state}
              connectAddress={ui.connectAddress}
              clientDownloadUrl={ui.clientDownloadUrl}
            />
          ))}
        </div>
        <script src="https://unpkg.com/htmx.org@2/dist/htmx.min.js" />
        <script src="https://unpkg.com/htmx-ext-sse@2/sse.js" />
        <script dangerouslySetInnerHTML={{ __html: initScript }} />
      </body>
    </html>
  );
  return "<!DOCTYPE html>" + page.toString();
}

export function renderRowHeader(
  game: string,
  state: CachedGameState
): string {
  const metaOnline = state.status === "online";
  const dot = state.status === "online" ? "🟢" : state.status === "starting" ? "🟡" : "⚫";
  // Return just the row-header div (htmx swaps outerHTML on the 30s poll)
  const frag = (
    <div
      id={`row-header-${game}`}
      class="row-header"
      hx-get={`/status?game=${game}`}
      hx-trigger="every 30s"
      hx-target={`#row-header-${game}`}
      hx-swap="outerHTML"
      onclick={`toggleRow('${game}')`}
    >
      <span class="status-dot">{dot}</span>
      <span class="game-name">{game}</span>
      <span class="row-meta">
        {metaOnline && state.hostname ? <span>{state.hostname}</span> : null}
        {metaOnline && state.map ? <span>{state.map}</span> : null}
        {metaOnline ? <span>{state.players} player{state.players !== 1 ? "s" : ""}</span> : null}
        {!metaOnline ? <span class={state.status}>{state.status}</span> : null}
      </span>
      <button class="expand-btn" id={`expand-btn-${game}`}>[expand ▼]</button>
    </div>
  );
  return frag.toString();
}
```

- [ ] **Step 2: Type-check**

```bash
cd launcher && npx tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add launcher/src/ui.tsx
git commit -m "feat: rewrite UI as full-width accordion rows with inline admin"
```

---

### Task 4: Update `app.ts` — wire cache, rewrite routes

**Files:**
- Modify: `launcher/src/app.ts`

The `createApp` function now receives a `GameCache` alongside the `Backend`. `GET /` renders the public page from cache. `GET /status` is a new public fragment endpoint. Start/stop ops update the cache immediately. The old passphrase-gated full-page-render logic is removed.

- [ ] **Step 1: Rewrite `launcher/src/app.ts`**

```typescript
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
    // Auth check for htmx validation requests (HX-Request header present, no game/operation)
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
        // Refresh cache immediately
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
    if (c.req.header("hx-request") && passphrase) {
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
```

- [ ] **Step 2: Type-check**

```bash
cd launcher && npx tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add launcher/src/app.ts
git commit -m "feat: wire GameCache into app, add public GET /status fragment endpoint"
```

---

### Task 5: Update `server.ts` — start cache, pass to app

**Files:**
- Modify: `launcher/src/server.ts`

- [ ] **Step 1: Update `launcher/src/server.ts`**

```typescript
import { serve } from "@hono/node-server";
import { createBackend } from "./backends/index.js";
import { createApp } from "./app.js";
import { GameCache } from "./cache.js";
import { log } from "./logger.js";

const PORT = parseInt(process.env.PORT ?? "3000", 10);
const BACKEND = process.env.BACKEND ?? "ecs";

const backend = createBackend();
const cache = new GameCache(backend);
const app = createApp(backend, cache);

cache.start();

serve({ fetch: app.fetch, port: PORT }, info => {
  log.info(`launcher listening on http://localhost:${info.port} (backend: ${BACKEND})`);
  const games = Object.keys(backend.getGames());
  log.info(`games: ${games.length > 0 ? games.join(", ") : "(none configured)"}`);
});
```

- [ ] **Step 2: Type-check and build**

```bash
cd launcher && npx tsc --noEmit && npm run build:docker
```

Expected: 0 errors, `dist/server.js` built.

- [ ] **Step 3: Smoke test**

```bash
PORT=3001 BACKEND=docker WEB_UI_PASSPHRASE=test API_TOKEN=test SIDECAR_TOKEN=test \
  GAMES='{"xonotic":{"containerName":"x","sidecarPort":5001,"image":"ghcr.io/fogo-sh/insta-game:xonotic","connectPort":26000,"clientDownloadUrl":"https://xonotic.org/download/"}}' \
  node launcher/dist/server.js &
sleep 1
curl -s http://localhost:3001/ | grep -o 'insta-game' | head -1
kill %1 2>/dev/null
```

Expected output: `insta-game`

- [ ] **Step 4: Commit**

```bash
git add launcher/src/server.ts launcher/dist/server.js
git commit -m "feat: start GameCache in server.ts, pass to createApp"
```

---

### Task 6: Update `compose.yml` — add `connectPort`, `clientDownloadUrl`, `PUBLIC_HOST`

**Files:**
- Modify: `compose.yml`

- [ ] **Step 1: Add `PUBLIC_HOST` to the launcher environment in `compose.yml`**

In the `launcher:` service `environment:` block, add after `SIDECAR_HOST`:

```yaml
      PUBLIC_HOST: "${PUBLIC_HOST:-localhost}"
```

- [ ] **Step 2: Add `connectPort` and `clientDownloadUrl` to each game in the `GAMES` JSON**

Update the `GAMES` value in the launcher service. The full updated block (replace the entire `GAMES: |` value):

```yaml
      GAMES: |
        {
          "xonotic": {
            "containerName": "insta-game-xonotic-1",
            "image": "ghcr.io/fogo-sh/insta-game:xonotic",
            "sidecarPort": 5001,
            "connectPort": 26000,
            "clientDownloadUrl": "https://xonotic.org/download/",
            "ports": {
              "26000/udp": {"hostPort": "26000"},
              "5001/tcp":  {"hostPort": "5001"}
            },
            "environment": {
              "PROTOCOL": "xonotic",
              "GAME_CMD": "./xonotic-linux-arm64-dedicated",
              "GAME_ARGS": "",
              "GAME_QUIT_CMD": "exit",
              "GAME_QUIT_TIMEOUT": "30",
              "TOKEN": "${SIDECAR_TOKEN:-abc123}"
            }
          },
          "qssm": {
            "containerName": "insta-game-qssm-1",
            "image": "ghcr.io/fogo-sh/insta-game:qssm",
            "sidecarPort": 5001,
            "connectPort": 26000,
            "clientDownloadUrl": "https://sourceforge.net/projects/fteqw/",
            "ports": {
              "26000/udp": {"hostPort": "26000"},
              "5001/tcp":  {"hostPort": "5001"}
            },
            "environment": {
              "PROTOCOL": "quake1",
              "GAME_CMD": "./qssm",
              "GAME_ARGS": "-dedicated 12 -basedir /opt -game id1 -port 26000 +exec server.cfg",
              "GAME_QUIT_CMD": "quit",
              "GAME_QUIT_TIMEOUT": "15",
              "CONFIG_PATH": "/opt/id1/server.cfg",
              "TOKEN": "${SIDECAR_TOKEN:-abc123}",
              "DATA_URL": "${QSSM_DATA_URL:-}"
            },
            "volumes": [".cache/qssm:/opt/id1"]
          },
          "q2repro": {
            "containerName": "insta-game-q2repro-1",
            "image": "ghcr.io/fogo-sh/insta-game:q2repro",
            "sidecarPort": 5001,
            "connectPort": 27910,
            "clientDownloadUrl": "https://github.com/Paril/q2repro/releases",
            "ports": {
              "27910/udp": {"hostPort": "27910"},
              "5001/tcp":  {"hostPort": "5003"}
            },
            "environment": {
              "PROTOCOL": "quake2",
              "GAME_CMD": "./q2proded",
              "GAME_ARGS": "+set dedicated 1 +set basedir /opt +set game baseq2 +set net_ip 0.0.0.0 +set net_port 27910 +set maxclients 4 +exec server.cfg",
              "GAME_QUIT_CMD": "quit",
              "GAME_QUIT_TIMEOUT": "15",
              "GAME_PORT": "27910",
              "CONFIG_PATH": "/opt/baseq2/server.cfg",
              "TOKEN": "${SIDECAR_TOKEN:-abc123}",
              "DATA_URL": "${Q2REPRO_DATA_URL:-}"
            },
            "volumes": [".cache/q2repro:/opt/baseq2"]
          },
          "bzflag": {
            "containerName": "insta-game-bzflag-1",
            "image": "ghcr.io/fogo-sh/insta-game:bzflag",
            "sidecarPort": 5001,
            "connectPort": 5154,
            "clientDownloadUrl": "https://www.bzflag.org/downloads/",
            "ports": {
              "5154/tcp":  {"hostPort": "5154"},
              "5154/udp":  {"hostPort": "5154"},
              "5001/tcp":  {"hostPort": "5004"}
            },
            "environment": {
              "PROTOCOL": "bzflag",
              "GAME_CMD": "/usr/games/bzfs",
              "GAME_ARGS": "-conf /opt/data/server.cfg -p 5154",
              "GAME_QUIT_CMD": "quit",
              "GAME_QUIT_TIMEOUT": "15",
              "GAME_PORT": "5154",
              "CONFIG_PATH": "/opt/data/server.cfg",
              "TOKEN": "${SIDECAR_TOKEN:-abc123}"
            },
            "volumes": [".cache/bzflag:/opt/data"]
          },
          "ut99": {
            "containerName": "insta-game-ut99-1",
            "image": "ghcr.io/fogo-sh/insta-game:ut99",
            "sidecarPort": 5001,
            "connectPort": 7777,
            "clientDownloadUrl": "https://github.com/OldUnreal/UnrealTournamentPatches/releases",
            "ports": {
              "7777/udp": {"hostPort": "7777"},
              "7778/udp": {"hostPort": "7778"},
              "5001/tcp": {"hostPort": "5005"}
            },
            "environment": {
              "PROTOCOL": "ut99",
              "GAME_CMD": "/usr/local/bin/start-ut99.sh",
              "GAME_ARGS": "/opt/SystemARM64/ucc-bin-arm64 server DM-Deck16][?game=Botpack.DeathMatchPlus ini=/opt/data/UnrealTournament.ini -nohomedir",
              "GAME_QUIT_CMD": "exit",
              "GAME_QUIT_TIMEOUT": "15",
              "GAME_PORT": "7777",
              "CONFIG_PATH": "/opt/data/UnrealTournament.ini",
              "TOKEN": "${SIDECAR_TOKEN:-abc123}",
              "DATA_URL": "${UT99_DATA_URL:-}"
            },
            "volumes": [".cache/ut99:/opt"]
          }
        }
```

- [ ] **Step 3: Build and verify compose config is valid**

```bash
docker compose config --quiet && echo "compose config ok"
```

Expected: `compose config ok`

- [ ] **Step 4: Commit**

```bash
git add compose.yml
git commit -m "feat: add connectPort, clientDownloadUrl, PUBLIC_HOST to compose config"
```

---

### Task 7: Build container and verify end-to-end

- [ ] **Step 1: Build the launcher container**

```bash
./build.sh launcher
```

Expected: image builds with 0 errors.

- [ ] **Step 2: Restart the launcher**

```bash
docker compose up -d --build launcher
```

- [ ] **Step 3: Verify public page loads without auth**

```bash
curl -s http://localhost:3000/ | grep -o 'insta-game'
```

Expected: `insta-game`

- [ ] **Step 4: Verify `/status` fragment endpoint**

```bash
curl -s "http://localhost:3000/status?game=xonotic"
```

Expected: HTML containing `row-header-xonotic`

- [ ] **Step 5: Verify auth still works**

```bash
curl -s -o /dev/null -w "%{http_code}" \
  -H "X-Passphrase: wrongpassword" \
  "http://localhost:3000/?game=xonotic&operation=status"
```

Expected: `401`

- [ ] **Step 6: Commit**

```bash
git add launcher/dist/server.js
git commit -m "chore: rebuild docker bundle for public status page"
```
