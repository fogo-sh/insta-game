# Design: Lambda Rewrite — TypeScript/Hono with Discord + Web UI

**Date:** 2026-04-03

## Goal

Replace the Python `launcher.py` Lambda with a TypeScript Lambda using Hono as the HTTP router. The new Lambda handles three concerns in one function: Discord slash command interactions (webhook mode), a passphrase-gated web UI for humans, and a token-protected JSON API for programmatic use.

## File Structure

```
lambda/launcher/
  src/
    index.ts          # Hono app entry point, route wiring, Lambda adapter
    discord.ts        # Discord interaction handler + Ed25519 signature verification
    games.ts          # ECS start/stop/status logic (port of launcher.py)
    ui.ts             # Web UI HTML (inline template string)
  register-commands.ts  # Standalone script to register Discord slash commands globally
  package.json
  tsconfig.json
```

The old `launcher.py` is deleted.

## Routes

| Route | Method | Auth | Purpose |
|---|---|---|---|
| `/` | GET | none | Serves the web UI page |
| `/` | POST | `X-Passphrase` header | Web UI form action — start/stop/status |
| `/api` | GET | `X-API-Token` header | JSON API — `?game=&operation=` |
| `/discord` | POST | Discord Ed25519 signature | Discord interaction webhook |

All unmatched routes return 404.

## `games.ts` — Game Control Logic

Port of `launcher.py` to TypeScript using the AWS SDK v3. Exports:

```typescript
interface GameConfig {
  serviceName: string;
  sidecarPort: number;
}

interface GameState {
  status: "offline" | "starting" | "online";
  publicIp?: string;
  players: number;
  ready: boolean;
  configUrl?: string;
}

const GAMES: Record<string, GameConfig> = { ... } // from env

async function startGame(config: GameConfig, configUrl?: string): Promise<GameState>
async function stopGame(config: GameConfig): Promise<GameState>
async function getGameState(config: GameConfig): Promise<GameState>
```

`startGame` kills any existing instance, scales to 1, polls sidecar `/status` every 5s up to 10 times until ready. `stopGame` scales to 0, waits for offline. `getGameState` returns current ECS + sidecar state.

AWS clients (`ECSClient`, `EC2Client`) are instantiated once at module load.

## `discord.ts` — Discord Interaction Handler

Verifies the `X-Signature-Ed25519` and `X-Signature-Timestamp` headers using the `discord-interactions` npm package (or native `crypto.subtle` — no heavy deps). Rejects invalid signatures with 401.

Handles interaction types:
- `PING` (type 1) → returns `{ type: 1 }` (pong)
- `APPLICATION_COMMAND` (type 2) → dispatches to command handlers

Commands:
- `/status <game>` — calls `getGameState`, returns immediate response with status/IP/players
- `/stop <game>` — calls `stopGame`, returns immediate response
- `/start <game>` — returns `DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE` (type 5) immediately, then calls `startGame` in the background and POSTs the result to Discord's followup webhook (`https://discord.com/api/v10/webhooks/<appId>/<interactionToken>/messages/@original`)

The `<game>` option is a required string with choices: `xonotic`, `qssm`, `q2repro`.

## `ui.ts` — Web UI

Returns a complete HTML page as a string. No external assets, no CDN, no frameworks — pure inline HTML/CSS/JS.

**Unauthenticated state:** Shows a passphrase input and submit button. On submit, stores the passphrase in `sessionStorage` and reloads.

**Authenticated state:** Shows three panels, one per game (xonotic, qssm, q2repro), each with:
- Current status (polled via `fetch('/api?game=<name>')` with `X-API-Token` header, every 5s)
- Start / Stop buttons (POST to `/` with `X-Passphrase` header)
- Public IP and player count when online

Auth check: if `sessionStorage` has a passphrase, include it in all requests. If any request returns 401, clear `sessionStorage` and show the passphrase input again.

## `index.ts` — Hono App + Lambda Adapter

```typescript
import { Hono } from "hono";
import { handle } from "hono/aws-lambda";

const app = new Hono();

app.get("/", uiHandler);
app.post("/", passphraseAuth, gameActionHandler);
app.get("/api", tokenAuth, apiHandler);
app.post("/discord", discordHandler);

export const handler = handle(app);
```

`passphraseAuth` middleware checks `X-Passphrase` header against `WEB_UI_PASSPHRASE` env var, returns 401 if missing/wrong.

`tokenAuth` middleware checks `X-API-Token` header against `API_TOKEN` env var, returns 401 if missing/wrong.

## `register-commands.ts` — Command Registration Script

Standalone script, not bundled with the Lambda. Run once manually:

```sh
npx ts-node register-commands.ts
```

Requires `DISCORD_BOT_TOKEN` and `DISCORD_APP_ID` env vars (from `.env` or shell). Calls `PUT /applications/<appId>/commands` to register all three commands globally. Idempotent — safe to re-run.

## Pulumi Changes (`pulumi/__main__.py`)

Lambda runtime changes from `python3.12` to `nodejs22.x`. Code changes from a single `FileAsset` to a directory bundle. New env vars added:

```python
runtime="nodejs22.x",
handler="index.handler",
code=pulumi.AssetArchive({
    ".": pulumi.FileArchive("../lambda/launcher/dist")
}),
```

New Pulumi config keys (all secrets):

| Key | Purpose |
|---|---|
| `webUiPassphrase` | Passphrase for the web UI |
| `apiToken` | Token for `X-API-Token` JSON API auth |
| `discordPublicKey` | Discord app public key for signature verification |
| `discordBotToken` | Discord bot token for sending followup messages |
| `discordAppId` | Discord application ID |

`SIDECAR_TOKEN`, `ECS_CLUSTER`, and `GAMES` env vars are unchanged.

The build step (`npm run build`) must run before `pulumi up` to produce `dist/`. This is documented but not automated in Pulumi — the developer runs it manually, same as `make download` for Xonotic.

## Build

`package.json` includes an `esbuild` build script that bundles `src/index.ts` to `dist/index.js` (CommonJS, targeting Node 22, external: `@aws-sdk/*` since those are available in the Lambda runtime).

```json
{
  "scripts": {
    "build": "esbuild src/index.ts --bundle --platform=node --target=node22 --outfile=dist/index.js --external:@aws-sdk/*"
  }
}
```

## Deletions

- `lambda/launcher/launcher.py`

## Out of Scope

- No automated CI for the Lambda build (manual `npm run build` before deploy)
- No persistent Discord bot (webhook mode only — Lambda is stateless)
- No rate limiting on the web UI or API
- No per-user Discord permissions (any Discord user in the server can run commands)
