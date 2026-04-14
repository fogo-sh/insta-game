# Preact Frontend Migration Design

**Date:** 2026-04-06
**Status:** Approved

## Goal

Replace the current hybrid SSR+string-blob-JS frontend with a Preact SPA. The server becomes a pure JSON API. The client is a compiled Preact bundle served as a static file. This eliminates the duplicate renderer problem, gives the client full type safety, and makes the codebase easier to maintain.

---

## Background

The current frontend has two rendering paths that must stay in sync:
- `renderRowHeaderContent()` in `ui-shared.ts` (server-side TypeScript)
- `renderRowHeader()` inside the `initScript` string blob in `ui-client.ts` (untyped JS-in-a-string)

Both build identical HTML. Any change to one requires a matching manual change to the other, with no type safety catching drift. The `ui-client.ts` blob (275 lines) is not type-checked and has no IDE support.

The htmx parts (start/stop buttons, SSE log panel) are fine, but the custom status polling loop and auth state management are fighting htmx's model. Preact handles these naturally.

---

## Architecture

**Clean split: server owns data, client owns rendering.**

### Server (`app.ts`)

Serves a static HTML shell at `GET /` and a compiled JS bundle at `GET /client.js`. All other routes return JSON.

Routes that change:
- `GET /` — returns 10-line HTML shell, no JSX, no game data
- `GET /client.js` — serves `dist/client.js` (compiled Preact bundle)
- `GET /status` — returns full status+metadata JSON object for all games (see API shape)
- `GET /logs` — adds `X-Log-Mode: sse | poll` response header so client knows which mode

Routes that are unchanged:
- `POST /` — start/stop, returns `{ status, publicIp, players, ready }`
- `GET /api` — JSON API for external callers
- `POST /discord` — Discord webhook

Files deleted from `launcher/src/`:
- `ui.tsx` (barrel re-export)
- `ui-render.tsx` (server JSX)
- `ui-shared.ts` (shared ID helpers + duplicate renderer)
- `ui-styles.ts` (CSS string)
- `ui-client.ts` (JS string blob)

### Client (`launcher/src/client/`)

Four files:

| File | Responsibility |
|---|---|
| `index.tsx` | Entry point — mounts `<App />` into `<div id="app">` |
| `App.tsx` | Auth state, game list, status polling loop |
| `GameRow.tsx` | Accordion row — open/closed, start/stop actions, log panel |
| `LogPanel.tsx` | Log viewer — SSE or poll mode |
| `api.ts` | Typed fetch wrappers for all server endpoints |

---

## API Shape

### `GET /status`

Returns all games in one object. Each entry contains both live cache state and static UI metadata:

```typescript
type StatusResponse = Record<string, {
  // live state from GameCache
  status: "online" | "starting" | "offline";
  players: number;
  hostname: string;
  map: string;
  // static ui metadata from GameConfig
  displayName: string;
  connectAddress: string | null;   // e.g. "games.fogo.sh:26000", null if no connectPort
  clientDownloadUrl: string | null;
  startBlocked: boolean;           // true if a conflicting game owns this port
}>
```

### `POST /?game=xonotic&operation=start`

Header: `X-Passphrase: <passphrase>`

Response (unchanged):
```typescript
{ status: "online" | "starting" | "offline"; publicIp: string; players: number; ready: boolean }
```

### `GET /` passphrase validation ping

Request: `GET /` with header `X-Passphrase: <passphrase>` and `X-Validate: true` (no game/operation params)
Response: `200 "ok"` or `401 "unauthorized"`

### `GET /logs?game=xonotic&token=abc`

Adds response header `X-Log-Mode: sse` (Docker) or `X-Log-Mode: poll` (Lambda/ECS).

**SSE mode:** `text/event-stream`, events with `event: log`, `data: <plain text line>`. The sidecar's ANSI→HTML conversion (via `terminal.Render`) means data may contain `<span>` tags — rendered with `innerHTML` in the client.

**Poll mode:** `GET /logs?game=xonotic&token=abc&cursor=<nextToken>`
```typescript
{ lines: string[]; cursor: string }
```

### HTML shell (`GET /`)

```html
<!DOCTYPE html>
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
</html>
```

---

## State Management

No external store. All state in Preact hooks.

### `App.tsx`
- `passphrase: string | null` — loaded from `sessionStorage` on mount. `null` = not authenticated.
- `games: StatusResponse | null` — fetched from `GET /status` on mount and every 10s.
- `authError: boolean` — set to `true` when passphrase validation returns 401.
- `pauseUntil: number` (ref, not state) — timestamp. Polling skips if `Date.now() < pauseUntil`. Set to `Date.now() + 15000` after any start/stop action.

Polling behaviour:
- `setInterval` at 10s in a `useEffect`
- Checks `pauseUntil` ref before each fetch
- On 429 response: extends interval to 30s for next cycle, then returns to 10s
- On successful auth: stores passphrase in `sessionStorage`, sets `passphrase` state

### `GameRow.tsx`
- `open: boolean` — accordion expanded
- `logsOpen: boolean` — log panel visible
- `actionResult: { message: string; ok: boolean } | null` — last start/stop outcome, shown below admin controls, cleared on next action

### `LogPanel.tsx`
- `lines: string[]` — accumulated log lines (SSE appends; poll replaces/appends with cursor)
- `connected: boolean` — whether the connection is active
- `cursor: string` — CloudWatch nextToken for poll mode, empty string initially

---

## Component Tree

```
App
├── <header>
│   ├── <h1>insta-game</h1>
│   └── passphrase form (if !passphrase) | "admin" label (if passphrase)
└── <main>
    └── GameRow × N (one per game, sorted by displayName)
        ├── row header (always visible)
        │   ├── status dot (🟢 🟡 ⚫)
        │   ├── displayName
        │   ├── row-meta: hostname, map, players (if online) | status label (if not)
        │   └── [expand ▼] button (hidden if offline AND !passphrase)
        └── row body (if open)
            ├── connect address + copy button (if connectAddress set)
            ├── get client link (if clientDownloadUrl set)
            ├── admin controls (if passphrase)
            │   ├── [start] (disabled if startBlocked)
            │   ├── [stop]
            │   ├── [logs]
            │   └── actionResult message
            └── LogPanel (if logsOpen && passphrase)
                └── scrollable div of log lines
```

---

## Build Setup

Add to `launcher/package.json`:

```json
"build:client": "esbuild src/client/index.tsx --bundle --platform=browser --target=es2020 --outfile=dist/client.js --jsx=automatic --jsx-import-source=preact"
```

Update `build:docker` to run both:
```json
"build:docker": "npm run build:client && esbuild src/server.ts --bundle --platform=node --target=node22 --outfile=dist/server.js"
```

Lambda build (`build`) also needs the client:
```json
"build": "npm run build:client && esbuild src/index.ts --bundle --platform=node --target=node22 --outfile=dist/index.js --external:@aws-sdk/*"
```

Add `preact` to dependencies:
```
npm install preact
```

The Dockerfile copies `dist/` into the image — `dist/client.js` will be picked up automatically.

---

## Styling

CSS moves from `ui-styles.ts` into `src/client/styles.css`, imported by `index.tsx`. esbuild handles CSS bundling with `--bundle` — or inline as a `<style>` tag injected by the entry point if we want zero extra files. The existing CSS classes and visual design are preserved exactly — this is a code structure change, not a visual redesign.

---

## Files Changed Summary

| File | Action |
|---|---|
| `launcher/src/ui.tsx` | Delete |
| `launcher/src/ui-render.tsx` | Delete |
| `launcher/src/ui-shared.ts` | Delete |
| `launcher/src/ui-styles.ts` | Delete |
| `launcher/src/ui-client.ts` | Delete |
| `launcher/src/app.ts` | Rewrite — remove all UI rendering, update `/status` to return full JSON, add `/client.js` route, simplify `GET /` to return HTML shell |
| `launcher/src/client/index.tsx` | Create |
| `launcher/src/client/App.tsx` | Create |
| `launcher/src/client/GameRow.tsx` | Create |
| `launcher/src/client/LogPanel.tsx` | Create |
| `launcher/src/client/api.ts` | Create |
| `launcher/src/client/styles.css` | Create |
| `launcher/package.json` | Add `preact`, update build scripts |

---

## Out of Scope

- Visual redesign — preserve existing look exactly
- New features
- Any changes to server routes beyond what's described above
- React Router or any other routing library — single page, no routes needed
