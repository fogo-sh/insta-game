# Public Status Page Design

**Date:** 2026-04-04  
**Status:** Approved  

## Goal

Redesign `GET /` on the launcher so unauthenticated visitors can see the live state of all game servers, get connect addresses, and find client download links — while authenticated admins get the same page with start/stop/logs controls revealed inline.

---

## Architecture

`GET /` is the single public URL. It always renders — no passphrase required. The page has two layers:

- **Public layer** (always visible): accordion rows per game showing status, players, map, hostname, connect address, download link. Auto-refreshes every 30s via htmx.
- **Admin layer** (revealed after passphrase): start/stop buttons and log viewer expand inside each accordion row.

### Server-side state cache

The launcher maintains an in-memory cache:

```ts
Map<string, {
  status: "offline" | "starting" | "online";
  players: number;
  hostname: string;
  map: string;
  updatedAt: Date;
}>
```

A background poller runs every 30s on startup. For each configured game it fetches `http://{SIDECAR_HOST}:{hostPort}/status` directly (the sidecar endpoint requires no auth and returns `hostname`, `map`, `players`, `running`, `ready`). Results are written to the cache.

- Cache starts empty; games show as offline until the first poll completes (typically within a second of startup).
- If a sidecar is unreachable the cache entry is set to offline.
- Authenticated start/stop operations update the cache immediately on completion so the public view reflects changes without waiting for the next 30s cycle.

### New public endpoint

`GET /status?game=<key>` — no auth required. Returns an HTML fragment (the accordion row content) read from the cache. Used by htmx `hx-trigger="every 30s"` on each row to keep the public view current.

---

## UI: Full-width accordion rows

Each game is a full-width accordion. All games are listed on the page regardless of auth state.

### Collapsed row (online)

```
┌──────────────────────────────────────────────────────────────────┐
│ 🟢 Xonotic   Insta Game FFA   glowplant   3 players   [expand ▼] │
└──────────────────────────────────────────────────────────────────┘
```

### Collapsed row (offline)

```
┌──────────────────────────────────────────────────────────────────┐
│ ⚫ Xonotic   offline                                  [expand ▼] │
└──────────────────────────────────────────────────────────────────┘
```

### Expanded row (unauthenticated)

```
┌──────────────────────────────────────────────────────────────────┐
│ 🟢 Xonotic   Insta Game FFA   glowplant   3 players [collapse ▲] │
├──────────────────────────────────────────────────────────────────┤
│  connect: games.fogo.sh:26000              [get client ↗]        │
│                                                                  │
│  [admin ▼]                                                       │
└──────────────────────────────────────────────────────────────────┘
```

### Expanded row (authenticated)

```
┌──────────────────────────────────────────────────────────────────┐
│ 🟢 Xonotic   Insta Game FFA   glowplant   3 players [collapse ▲] │
├──────────────────────────────────────────────────────────────────┤
│  connect: games.fogo.sh:26000              [get client ↗]        │
│                                                                  │
│  [start]  [stop]  [logs]                                         │
└──────────────────────────────────────────────────────────────────┘
```

### Behaviour notes

- Accordion open/close is pure JS — no server round-trip needed.
- The 30s htmx poll updates the collapsed row's status indicator in place, even when collapsed, so the summary line always reflects current state.
- Connect address is displayed as a copyable `<code>` element.
- "get client ↗" opens `clientDownloadUrl` in a new tab. Hidden if `clientDownloadUrl` is not configured for the game.
- Connect address is hidden if `connectPort` is not configured.
- "admin ▼" button is always visible but visually muted. Clicking it when unauthenticated opens the passphrase prompt inline within the accordion. Once authenticated the prompt is replaced by start/stop/logs controls.
- The passphrase is validated against the server before being stored (existing behaviour).

---

## Data flow

```
startup
  └─> startCachePoller()
        every 30s:
          for each game in backend.getGames():
            hostPort = inspect container → get sidecar host port
            fetch http://{SIDECAR_HOST}:{hostPort}/status
            → cache[game] = { status, players, hostname, map, updatedAt }

GET / (no auth required)
  └─> read all games from backend.getGames()
  └─> read state for each from cache (stale-ok, default offline)
  └─> render full page: accordion rows with public layer visible

GET /status?game=<key> (no auth required, htmx fragment endpoint)
  └─> read from cache
  └─> return collapsed row HTML fragment

POST /?game=&operation=start|stop (auth required, existing flow)
  └─> backend.startGame / stopGame (hits Docker/ECS directly)
  └─> update cache entry immediately on completion
```

---

## New config fields

Two new optional fields in the `GAMES` JSON per game entry:

| Field | Type | Description |
|---|---|---|
| `connectPort` | `number` | Port players use to connect. Shown as `PUBLIC_HOST:connectPort`. Hidden if absent. |
| `clientDownloadUrl` | `string` | URL to the game's client download page. Powers the "get client" link. Hidden if absent. |

These fields flow through the existing `[key: string]: unknown` passthrough in `GameConfig` — no interface changes required.

### Example GAMES entry

```json
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
    "environment": { "...": "..." }
  }
}
```

### Suggested `clientDownloadUrl` values per game

| Game | URL |
|---|---|
| xonotic | `https://xonotic.org/download/` |
| fteqw / qssm | `https://sourceforge.net/projects/fteqw/` |
| q2repro | `https://github.com/Paril/q2repro/releases` |
| bzflag | `https://www.bzflag.org/downloads/` |
| ut99 | `https://github.com/OldUnreal/UnrealTournamentPatches/releases` |

---

## New env var

| Var | Default | Description |
|---|---|---|
| `PUBLIC_HOST` | `localhost` | Hostname shown in connect addresses on the public page. Set to e.g. `games.fogo.sh` for a public-facing deployment. |

---

## Files to change

| File | Change |
|---|---|
| `launcher/src/app.ts` | Add `GET /status` fragment endpoint; rewrite `GET /` to render public view; start cache poller on init; update cache after start/stop |
| `launcher/src/ui.tsx` | Replace grid of GameCards with full-width accordion rows; split public/admin layers; add inline passphrase prompt per-row |
| `launcher/src/cache.ts` | New file — `GameCache` class with `start()` poller, `get()`, `set()` |
| `launcher/src/backend.ts` | Add optional `getCachedState(config: GameConfig): Promise<CachedGameState>` to the `Backend` interface for the cache poller to use |
| `launcher/src/backends/docker.ts` | Implement `getCachedState`: inspect container → get host sidecar port → fetch `/status` → return full fields including `hostname` and `map` |
| `launcher/src/backends/ecs.ts` | Implement `getCachedState`: use existing `getGameState` flow to get `publicIp`, then fetch sidecar `/status` directly for `hostname` and `map` |
| `compose.yml` | Add `connectPort`, `clientDownloadUrl`, `PUBLIC_HOST` to launcher service and GAMES entries |

---

## Out of scope for this spec

- Download links page (separate spec)
- Go CLI tool (separate spec)
- Any authentication changes beyond what's described above
