# Preact Frontend Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hybrid SSR+string-blob-JS frontend with a Preact SPA, making the server a pure JSON API and the client a compiled bundle.

**Architecture:** The Hono server serves a static HTML shell at `GET /` and a compiled `dist/client.js` at `GET /client.js`. All game data comes from `GET /status` (JSON). A Preact app in `src/client/` handles rendering, auth state, polling, and log streaming. All `ui-*.ts/tsx` files are deleted.

**Tech Stack:** TypeScript, Preact, esbuild (browser bundle), Hono (server JSON API), native `EventSource` for SSE logs, `setInterval` for status polling.

---

### Task 1: Install Preact, add build scripts

**Files:**
- Modify: `launcher/package.json`

- [ ] **Step 1: Install preact**

```bash
cd launcher && npm install preact
```

- [ ] **Step 2: Update `launcher/package.json` scripts**

Replace the `scripts` block with:

```json
"scripts": {
  "build:client": "esbuild src/client/index.tsx --bundle --platform=browser --target=es2020 --outfile=dist/client.js --jsx=automatic --jsx-import-source=preact",
  "build": "npm run build:client && esbuild src/index.ts --bundle --platform=node --target=node22 --outfile=dist/index.js --external:@aws-sdk/*",
  "build:watch": "esbuild src/index.ts --bundle --platform=node --target=node22 --outfile=dist/index.js --external:@aws-sdk/* --watch",
  "build:docker": "npm run build:client && esbuild src/server.ts --bundle --platform=node --target=node22 --outfile=dist/server.js",
  "lint": "eslint .",
  "register": "tsx register-commands.ts",
  "start": "node dist/server.js"
}
```

- [ ] **Step 3: Verify preact is in package.json dependencies**

```bash
cd launcher && grep '"preact"' package.json
```

Expected output: `"preact": "^10.x.x"` (or similar)

- [ ] **Step 4: Commit**

```bash
git add launcher/package.json launcher/package-lock.json
git commit -m "chore: add preact, update build scripts"
```

---

### Task 2: Create `api.ts` — typed server fetch wrappers

**Files:**
- Create: `launcher/src/client/api.ts`

- [ ] **Step 1: Create `launcher/src/client/api.ts`**

```typescript
export type GameStatus = "online" | "starting" | "offline";

export interface GameEntry {
  status: GameStatus;
  players: number;
  hostname: string;
  map: string;
  displayName: string;
  connectAddress: string | null;
  clientDownloadUrl: string | null;
  startBlocked: boolean;
}

export type StatusResponse = Record<string, GameEntry>;

export interface ActionResult {
  status: GameStatus;
  publicIp: string;
  players: number;
  ready: boolean;
}

export interface LogPollResult {
  lines: string[];
  cursor: string | null;
}

export async function fetchStatus(): Promise<StatusResponse> {
  const res = await fetch("/status");
  if (!res.ok) throw new Error(`/status returned ${res.status}`);
  return res.json() as Promise<StatusResponse>;
}

export async function validatePassphrase(passphrase: string): Promise<boolean> {
  const res = await fetch("/", {
    headers: { "X-Passphrase": passphrase, "X-Validate": "true" },
  });
  return res.ok;
}

export async function postAction(
  game: string,
  operation: "start" | "stop",
  passphrase: string
): Promise<ActionResult> {
  const res = await fetch(`/?game=${encodeURIComponent(game)}&operation=${operation}`, {
    method: "POST",
    headers: { "X-Passphrase": passphrase },
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `${operation} returned ${res.status}`);
  }
  return res.json() as Promise<ActionResult>;
}

export async function fetchLogMode(game: string, token: string): Promise<"sse" | "poll"> {
  const res = await fetch(`/logs?game=${encodeURIComponent(game)}&token=${encodeURIComponent(token)}`, {
    method: "HEAD",
  });
  const mode = res.headers.get("X-Log-Mode");
  return mode === "poll" ? "poll" : "sse";
}

export async function pollLogs(
  game: string,
  token: string,
  cursor: string | null
): Promise<LogPollResult> {
  const params = new URLSearchParams({ game, token });
  if (cursor) params.set("cursor", cursor);
  const res = await fetch(`/logs?${params}`);
  if (!res.ok) throw new Error(`/logs returned ${res.status}`);
  return res.json() as Promise<LogPollResult>;
}
```

- [ ] **Step 2: Type-check (client file only — can't run tsc on client until tsconfig updated, skip for now)**

We'll verify compilation in Task 3 when tsconfig is updated.

- [ ] **Step 3: Commit**

```bash
git add launcher/src/client/api.ts
git commit -m "feat(preact): add typed API client"
```

---

### Task 3: Create CSS and entry point, update tsconfig

**Files:**
- Create: `launcher/src/client/styles.css`
- Create: `launcher/src/client/index.tsx`
- Modify: `launcher/tsconfig.json`

- [ ] **Step 1: Read current tsconfig**

```bash
cat launcher/tsconfig.json
```

- [ ] **Step 2: Update `launcher/tsconfig.json`**

Add `"src/client/**"` to the include paths and configure Preact JSX. The full file should look like:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "lib": ["ES2022", "DOM"],
    "jsx": "react-jsx",
    "jsxImportSource": "preact",
    "strict": true,
    "skipLibCheck": true,
    "outDir": "dist",
    "rootDir": "src"
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
```

- [ ] **Step 3: Create `launcher/src/client/styles.css`**

Copy the exact CSS from the current `ui-styles.ts` export (strip the backtick string wrapper, keep the raw CSS). The full content:

```css
* { box-sizing: border-box; margin: 0; padding: 0; }
body { background: #111; color: #eee; font-family: monospace; padding: 2rem; }
.title-bar { display: flex; align-items: center; gap: 1.5rem; margin-bottom: 2rem; flex-wrap: wrap; }
h1 { font-size: 1.4rem; }
#auth-form { display: flex; align-items: center; gap: 0.5rem; }
#auth-form input { padding: 0.35rem 0.5rem; background: #222; color: #eee; border: 1px solid #444; font-family: monospace; font-size: 0.85rem; width: 12rem; }
#auth-form button { padding: 0.35rem 0.7rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; font-size: 0.85rem; }
#auth-status { font-size: 0.8rem; color: #aaa; }

.accordion { display: flex; flex-direction: column; gap: 0.5rem; }

.row { border: 1px solid #333; background: #1a1a1a; }
.row-header {
  display: flex; align-items: center; gap: 1rem;
  padding: 0.75rem 1rem; cursor: pointer; user-select: none;
  width: 100%;
}
.row-header.not-expandable { cursor: default; }
.row-header:not(.not-expandable):hover { background: #222; }
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
.connect { display: flex; align-items: center; gap: 0.4rem; }
.connect code { background: #222; padding: 0.2rem 0.5rem; border: 1px solid #444; user-select: text; cursor: text; }
.copy-btn { background: #333; border: 1px solid #555; color: #aaa; cursor: pointer; font-family: monospace; font-size: 0.75rem; padding: 0.15rem 0.4rem; line-height: 1; }
.copy-btn:hover { background: #444; color: #eee; }
.copy-btn.copied { color: #4f4; border-color: #4f4; }
.client-link { font-size: 0.85rem; color: #aaa; }
.client-link a { color: #88f; text-decoration: none; }
.client-link a:hover { text-decoration: underline; }

.admin-section { margin-top: 0.75rem; border-top: 1px solid #222; padding-top: 0.75rem; }
.admin-controls { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: center; }
.admin-controls button { padding: 0.4rem 0.8rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; }
.admin-controls button:hover { background: #444; }
.admin-controls button:disabled { opacity: 0.4; cursor: not-allowed; }

.action-result { font-size: 0.85rem; color: #aaa; margin-top: 0.5rem; }
.action-result.ok { color: #4f4; }
.action-result.err { color: #f44; }

.log-section { margin-top: 0.75rem; border-top: 1px solid #222; padding-top: 0.75rem; }
.log-panel { height: 300px; overflow-y: scroll; background: #0a0a0a; font-size: 0.75rem; padding: 0.75rem; border: 1px solid #222; }
.log-line { white-space: pre-wrap; word-break: break-all; line-height: 1.5; }
.term-fg1 { font-weight: bold; }
.term-fg2 { color: #838887; }
.term-fg3 { font-style: italic; }
.term-fg4 { text-decoration: underline; }
.term-fg30 { color: #666; }
.term-fg31 { color: #ff7070; }
.term-fg32 { color: #b0f986; }
.term-fg33 { color: #c6c502; }
.term-fg34 { color: #8db7e0; }
.term-fg35 { color: #f271fb; }
.term-fg36 { color: #6bf7ff; }
.term-fg37 { color: #eee; }
.term-fgi90 { color: #838887; }
.term-fgi91 { color: #ff3333; }
.term-fgi92 { color: #00ff00; }
.term-fgi93 { color: #fffc67; }
.term-fgi94 { color: #6871ff; }
.term-fgi95 { color: #ff76ff; }
.term-fgi96 { color: #60fcff; }

@media (max-width: 600px) {
  body { padding: 0.75rem; }
  .row-header {
    flex-wrap: wrap;
    padding: 0.75rem;
  }
  .game-name { min-width: 0; }
  .row-meta { width: 100%; gap: 0.5rem 1rem; }
  .expand-btn { margin-left: auto; }
  .row-body { padding: 0.75rem; }
  .row-details { gap: 0.75rem; margin-bottom: 0.75rem; }
  .connect { width: 100%; flex-wrap: wrap; }
  .connect code { max-width: calc(100% - 4rem); overflow-wrap: anywhere; }
  .admin-controls { gap: 0.75rem; }
}
```

- [ ] **Step 4: Create `launcher/src/client/index.tsx`**

```tsx
import { render } from "preact";
import { App } from "./App.js";
import "./styles.css";

const root = document.getElementById("app");
if (root) render(<App />, root);
```

- [ ] **Step 5: Type-check**

```bash
cd launcher && npx tsc --noEmit 2>&1 | head -20
```

Expected: errors only about missing `App` (not yet created) — no config errors.

- [ ] **Step 6: Commit**

```bash
git add launcher/tsconfig.json launcher/src/client/styles.css launcher/src/client/index.tsx
git commit -m "feat(preact): add CSS, entry point, update tsconfig"
```

---

### Task 4: Create `LogPanel.tsx`

**Files:**
- Create: `launcher/src/client/LogPanel.tsx`

- [ ] **Step 1: Create `launcher/src/client/LogPanel.tsx`**

```tsx
import { h } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import { pollLogs } from "./api.js";

interface LogPanelProps {
  game: string;
  passphrase: string;
}

export function LogPanel({ game, passphrase }: LogPanelProps) {
  const [lines, setLines] = useState<string[]>([`[connecting to ${game} logs...]`]);
  const [logMode, setLogMode] = useState<"sse" | "poll" | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const cursorRef = useRef<string | null>(null);
  const activeRef = useRef(true);

  // Auto-scroll when lines change
  useEffect(() => {
    const el = panelRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [lines]);

  // Detect log mode from server then start streaming
  useEffect(() => {
    activeRef.current = true;

    // HEAD request to get X-Log-Mode header
    fetch(`/logs?game=${encodeURIComponent(game)}&token=${encodeURIComponent(passphrase)}`, {
      method: "HEAD",
    }).then(res => {
      if (!activeRef.current) return;
      const mode = res.headers.get("X-Log-Mode") === "poll" ? "poll" : "sse";
      setLogMode(mode);
    }).catch(() => {
      if (activeRef.current) setLines(l => [...l, "[failed to connect]"]);
    });

    return () => { activeRef.current = false; };
  }, [game, passphrase]);

  // SSE mode
  useEffect(() => {
    if (logMode !== "sse") return;
    const url = `/logs?game=${encodeURIComponent(game)}&token=${encodeURIComponent(passphrase)}`;
    const es = new EventSource(url);

    es.addEventListener("log", (e: MessageEvent) => {
      if (!activeRef.current) return;
      setLines(l => [...l, e.data as string]);
    });

    es.onerror = () => {
      if (activeRef.current) setLines(l => [...l, "[log stream disconnected]"]);
      es.close();
    };

    return () => { es.close(); };
  }, [logMode, game, passphrase]);

  // Poll mode
  useEffect(() => {
    if (logMode !== "poll") return;
    let stopped = false;

    async function doPoll() {
      while (!stopped && activeRef.current) {
        try {
          const result = await pollLogs(game, passphrase, cursorRef.current);
          if (result.lines.length > 0) {
            setLines(l => [...l, ...result.lines]);
          }
          if (result.cursor) cursorRef.current = result.cursor;
        } catch (err) {
          if (!stopped) setLines(l => [...l, `[poll error: ${err instanceof Error ? err.message : String(err)}]`]);
        }
        await new Promise(r => setTimeout(r, 5000));
      }
    }

    void doPoll();
    return () => { stopped = true; };
  }, [logMode, game, passphrase]);

  return (
    <div class="log-panel" ref={panelRef}>
      {lines.map((line, i) => (
        <div
          key={i}
          class="log-line"
          // Lines from the sidecar may contain HTML spans from terminal.Render (ANSI colours)
          dangerouslySetInnerHTML={{ __html: line }}
        />
      ))}
    </div>
  );
}
```

- [ ] **Step 2: Type-check**

```bash
cd launcher && npx tsc --noEmit 2>&1 | grep -v "App"
```

Expected: 0 errors (ignoring missing App).

- [ ] **Step 3: Commit**

```bash
git add launcher/src/client/LogPanel.tsx
git commit -m "feat(preact): add LogPanel component"
```

---

### Task 5: Create `GameRow.tsx`

**Files:**
- Create: `launcher/src/client/GameRow.tsx`

- [ ] **Step 1: Create `launcher/src/client/GameRow.tsx`**

```tsx
import { h } from "preact";
import { useState, useCallback } from "preact/hooks";
import { postAction, type GameEntry } from "./api.js";
import { LogPanel } from "./LogPanel.js";

interface GameRowProps {
  id: string;
  game: GameEntry;
  passphrase: string | null;
  onAction: () => void; // called after start/stop to pause polling
}

function statusDot(status: string): string {
  if (status === "online") return "🟢";
  if (status === "starting") return "🟡";
  return "⚫";
}

export function GameRow({ id, game, passphrase, onAction }: GameRowProps) {
  const [open, setOpen] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [actionResult, setActionResult] = useState<{ message: string; ok: boolean } | null>(null);
  const [acting, setActing] = useState(false);

  const expandable = game.status !== "offline" || passphrase !== null;

  const toggle = useCallback(() => {
    if (!expandable) return;
    setOpen(o => !o);
  }, [expandable]);

  const handleAction = useCallback(async (operation: "start" | "stop") => {
    if (!passphrase) return;
    setActing(true);
    setActionResult(null);
    onAction();
    try {
      const result = await postAction(id, operation, passphrase);
      setActionResult({ message: `${operation} → ${result.status}`, ok: result.status !== "offline" });
    } catch (err) {
      setActionResult({ message: `${operation} failed: ${err instanceof Error ? err.message : String(err)}`, ok: false });
    } finally {
      setActing(false);
    }
  }, [id, passphrase, onAction]);

  const copyConnect = useCallback((address: string, e: MouseEvent) => {
    void navigator.clipboard.writeText(address);
    const btn = e.currentTarget as HTMLButtonElement;
    const orig = btn.textContent;
    btn.textContent = "✓";
    btn.classList.add("copied");
    setTimeout(() => {
      btn.textContent = orig;
      btn.classList.remove("copied");
    }, 1500);
  }, []);

  return (
    <div class="row">
      {/* Row header */}
      <div
        class={`row-header${expandable ? "" : " not-expandable"}`}
        onClick={toggle}
      >
        <span class="status-dot">{statusDot(game.status)}</span>
        <span class="game-name">{game.displayName || id}</span>
        <span class="row-meta">
          {game.status === "online" && game.hostname ? <span>{game.hostname}</span> : null}
          {game.status === "online" && game.map ? <span>{game.map}</span> : null}
          {game.status === "online" ? (
            <span>{game.players} player{game.players !== 1 ? "s" : ""}</span>
          ) : (
            <span class={game.status}>{game.status}</span>
          )}
        </span>
        {expandable ? (
          <button class="expand-btn" onClick={e => { e.stopPropagation(); toggle(); }}>
            {open ? "[collapse ▲]" : "[expand ▼]"}
          </button>
        ) : null}
      </div>

      {/* Row body */}
      {open ? (
        <div class="row-body open">
          <div class="row-details">
            {game.connectAddress ? (
              <div class="connect">
                connect: <code>{game.connectAddress}</code>
                <button
                  class="copy-btn"
                  title="copy to clipboard"
                  onClick={e => copyConnect(game.connectAddress!, e)}
                >copy</button>
              </div>
            ) : null}
            {game.clientDownloadUrl ? (
              <div class="client-link">
                <a href={game.clientDownloadUrl} target="_blank" rel="noopener">get client ↗</a>
              </div>
            ) : null}
          </div>

          {passphrase ? (
            <div class="admin-section">
              <div class="admin-controls">
                <button
                  onClick={() => void handleAction("start")}
                  disabled={acting || game.startBlocked}
                  title={game.startBlocked ? "a conflicting game is already running on the same port" : undefined}
                >start</button>
                <button
                  onClick={() => void handleAction("stop")}
                  disabled={acting}
                >stop</button>
                <button
                  type="button"
                  onClick={() => setLogsOpen(l => !l)}
                >{logsOpen ? "hide logs" : "logs"}</button>
              </div>
              {actionResult ? (
                <div class={`action-result ${actionResult.ok ? "ok" : "err"}`}>
                  {actionResult.message}
                </div>
              ) : null}
            </div>
          ) : null}

          {logsOpen && passphrase ? (
            <div class="log-section">
              <LogPanel game={id} passphrase={passphrase} />
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
```

- [ ] **Step 2: Type-check**

```bash
cd launcher && npx tsc --noEmit 2>&1 | grep -v "App"
```

Expected: 0 errors (ignoring missing App).

- [ ] **Step 3: Commit**

```bash
git add launcher/src/client/GameRow.tsx
git commit -m "feat(preact): add GameRow component"
```

---

### Task 6: Create `App.tsx`

**Files:**
- Create: `launcher/src/client/App.tsx`

- [ ] **Step 1: Create `launcher/src/client/App.tsx`**

```tsx
import { h } from "preact";
import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { fetchStatus, validatePassphrase, type StatusResponse } from "./api.js";
import { GameRow } from "./GameRow.js";

const SESSION_KEY = "insta-game-passphrase";
const POLL_INTERVAL_MS = 10_000;
const PAUSE_AFTER_ACTION_MS = 15_000;
const BACKOFF_INTERVAL_MS = 30_000;

export function App() {
  const [passphrase, setPassphrase] = useState<string | null>(() =>
    sessionStorage.getItem(SESSION_KEY)
  );
  const [passphraseInput, setPassphraseInput] = useState("");
  const [authError, setAuthError] = useState(false);
  const [authPending, setAuthPending] = useState(false);
  const [games, setGames] = useState<StatusResponse | null>(null);
  const pauseUntilRef = useRef(0);
  const nextIntervalRef = useRef(POLL_INTERVAL_MS);

  // Validate stored passphrase on mount
  useEffect(() => {
    const stored = sessionStorage.getItem(SESSION_KEY);
    if (!stored) return;
    validatePassphrase(stored).then(ok => {
      if (!ok) {
        sessionStorage.removeItem(SESSION_KEY);
        setPassphrase(null);
      }
    });
  }, []);

  // Status polling
  useEffect(() => {
    let cancelled = false;

    async function poll() {
      if (cancelled) return;
      if (Date.now() < pauseUntilRef.current) {
        setTimeout(poll, Math.max(1000, pauseUntilRef.current - Date.now()));
        return;
      }
      try {
        const data = await fetchStatus();
        if (!cancelled) {
          setGames(data);
          nextIntervalRef.current = POLL_INTERVAL_MS;
        }
      } catch (err) {
        // On rate-limit, back off
        const status = err instanceof Response ? err.status : 0;
        if (status === 429) nextIntervalRef.current = BACKOFF_INTERVAL_MS;
      }
      if (!cancelled) setTimeout(poll, nextIntervalRef.current);
    }

    void poll();
    return () => { cancelled = true; };
  }, []);

  const handleAuth = useCallback(async (e: Event) => {
    e.preventDefault();
    if (!passphraseInput) return;
    setAuthPending(true);
    setAuthError(false);
    const ok = await validatePassphrase(passphraseInput);
    setAuthPending(false);
    if (!ok) {
      setAuthError(true);
      return;
    }
    sessionStorage.setItem(SESSION_KEY, passphraseInput);
    setPassphrase(passphraseInput);
    setPassphraseInput("");
  }, [passphraseInput]);

  const pausePolling = useCallback(() => {
    pauseUntilRef.current = Date.now() + PAUSE_AFTER_ACTION_MS;
  }, []);

  const sortedGames = games
    ? Object.entries(games).sort(([, a], [, b]) =>
        (a.displayName || "").localeCompare(b.displayName || "")
      )
    : null;

  return (
    <div>
      <div class="title-bar">
        <h1>insta-game</h1>
        {passphrase === null ? (
          <form id="auth-form" onSubmit={handleAuth}>
            <input
              type="text"
              value={passphraseInput}
              onInput={e => { setPassphraseInput((e.target as HTMLInputElement).value); setAuthError(false); }}
              placeholder="passphrase"
              autocomplete="off"
              spellcheck={false}
              style={authError ? "border-color: #f44; letter-spacing: 0.15em;" : "letter-spacing: 0.15em;"}
            />
            <button type="submit" disabled={authPending}>
              {authPending ? "checking..." : "unlock"}
            </button>
          </form>
        ) : (
          <span id="auth-status">admin</span>
        )}
      </div>

      <div class="accordion">
        {sortedGames === null ? (
          <div style="color: #666; font-size: 0.85rem;">loading...</div>
        ) : sortedGames.length === 0 ? (
          <div style="color: #666; font-size: 0.85rem;">no games configured</div>
        ) : (
          sortedGames.map(([id, game]) => (
            <GameRow
              key={id}
              id={id}
              game={game}
              passphrase={passphrase}
              onAction={pausePolling}
            />
          ))
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Type-check**

```bash
cd launcher && npx tsc --noEmit 2>&1
```

Expected: 0 errors.

- [ ] **Step 3: Verify client bundle builds**

```bash
cd launcher && npm run build:client 2>&1
```

Expected: `dist/client.js` created, no errors.

- [ ] **Step 4: Commit**

```bash
git add launcher/src/client/App.tsx
git commit -m "feat(preact): add App component"
```

---

### Task 7: Rewrite `app.ts` — pure JSON API

**Files:**
- Modify: `launcher/src/app.ts`

Key changes:
- `GET /` returns HTML shell (no JSX, no game data) — or handles passphrase validation ping
- `GET /client.js` serves the compiled client bundle
- `GET /status` returns full `StatusResponse` JSON (state + metadata per game)
- `GET /logs` adds `X-Log-Mode` header; SSE data changes from `<div>` HTML to raw text
- `POST /` returns JSON (no more `statusFragment` HTML)
- Remove all imports of UI files

- [ ] **Step 1: Rewrite `launcher/src/app.ts`**

```typescript
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
  try {
    clientBundle = readFileSync(join(process.cwd(), "dist/client.js"));
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
    return new Response(clientBundle, {
      headers: { "Content-Type": "application/javascript" },
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

    // Legacy htmx action dispatch (game + operation query params) — kept for compatibility
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
```

- [ ] **Step 2: Type-check**

```bash
cd launcher && npx tsc --noEmit 2>&1
```

Expected: 0 errors.

- [ ] **Step 3: Build both bundles**

```bash
cd launcher && npm run build:docker 2>&1 | tail -5
```

Expected: both `dist/client.js` and `dist/server.js` built, 0 errors.

- [ ] **Step 4: Smoke test**

```bash
PORT=3001 BACKEND=docker WEB_UI_PASSPHRASE=test API_TOKEN=test SIDECAR_TOKEN=test \
  GAMES='{"xonotic":{"containerName":"x","sidecarPort":5001,"image":"x","displayName":"Xonotic"}}' \
  node launcher/dist/server.js &
sleep 1
# HTML shell
curl -s http://localhost:3001/ | grep '<div id="app">'
# Status JSON
curl -s http://localhost:3001/status | python3 -m json.tool | head -10
# Client bundle
curl -s -o /dev/null -w "%{http_code}" http://localhost:3001/client.js
kill %1 2>/dev/null
```

Expected:
- Line 1: `  <div id="app"></div>`
- Line 2: JSON with game keys
- Line 3: `200`

- [ ] **Step 5: Commit**

```bash
git add launcher/src/app.ts
git commit -m "feat(preact): rewrite app.ts as pure JSON API"
```

---

### Task 8: Delete old UI files, build full bundles, verify

**Files:**
- Delete: `launcher/src/ui.tsx`
- Delete: `launcher/src/ui-render.tsx`
- Delete: `launcher/src/ui-shared.ts`
- Delete: `launcher/src/ui-styles.ts`
- Delete: `launcher/src/ui-client.ts`

- [ ] **Step 1: Delete old UI files**

```bash
git rm launcher/src/ui.tsx \
        launcher/src/ui-render.tsx \
        launcher/src/ui-shared.ts \
        launcher/src/ui-styles.ts \
        launcher/src/ui-client.ts
```

- [ ] **Step 2: Type-check**

```bash
cd launcher && npx tsc --noEmit 2>&1
```

Expected: 0 errors.

- [ ] **Step 3: Build all bundles**

```bash
cd launcher && npm run build:docker 2>&1 && npm run build 2>&1 | tail -5
```

Expected: `dist/client.js`, `dist/server.js`, `dist/index.js` all built.

- [ ] **Step 4: Rebuild launcher Docker image**

```bash
cd /path/to/insta-game && docker compose up -d --build launcher
sleep 3
curl -s http://localhost:3000/ | grep '<div id="app">'
curl -s http://localhost:3000/client.js | head -c 50
```

Expected:
- `  <div id="app"></div>`
- First 50 chars of the Preact bundle (starts with `(()=>{` or similar minified JS)

- [ ] **Step 5: Commit**

```bash
git add launcher/dist/client.js launcher/dist/server.js launcher/dist/index.js
git commit -m "feat(preact): delete old UI files, ship Preact SPA"
```
