# Lambda Rewrite — TypeScript/Hono with Discord + Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Python Lambda with a TypeScript/Hono Lambda that serves a passphrase-gated web UI, a token-protected JSON API, and Discord slash command interactions in webhook mode.

**Architecture:** Single Hono app bundled with esbuild into `dist/index.js`, deployed as a Node 22 Lambda. Game control logic is ported from `launcher.py` using AWS SDK v3. Discord interactions are verified with Ed25519 and handled with deferred responses for slow operations. The web UI is an inline HTML template string with no external assets.

**Tech Stack:** TypeScript, Hono, esbuild, AWS SDK v3 (`@aws-sdk/client-ecs`, `@aws-sdk/client-ec2`), `discord-interactions` npm package, Node 22 Lambda runtime.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `lambda/launcher/src/index.ts` | Create | Hono app, route wiring, Lambda adapter export |
| `lambda/launcher/src/games.ts` | Create | ECS start/stop/status logic, port of launcher.py |
| `lambda/launcher/src/discord.ts` | Create | Discord signature verification, interaction dispatch |
| `lambda/launcher/src/ui.ts` | Create | Web UI HTML template |
| `lambda/launcher/register-commands.ts` | Create | One-off Discord command registration script |
| `lambda/launcher/package.json` | Create | Dependencies and build script |
| `lambda/launcher/tsconfig.json` | Create | TypeScript config |
| `lambda/launcher/launcher.py` | Delete | Replaced by TypeScript |
| `pulumi/__main__.py` | Modify | Runtime, handler, code path, new env vars |

---

### Task 1: Scaffold the TypeScript project

**Files:**
- Create: `lambda/launcher/package.json`
- Create: `lambda/launcher/tsconfig.json`

- [ ] **Step 1: Write package.json**

```bash
mkdir -p lambda/launcher/src
```

Write `lambda/launcher/package.json`:

```json
{
  "name": "insta-game-launcher",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "build": "esbuild src/index.ts --bundle --platform=node --target=node22 --outfile=dist/index.js --external:@aws-sdk/*",
    "build:watch": "esbuild src/index.ts --bundle --platform=node --target=node22 --outfile=dist/index.js --external:@aws-sdk/* --watch"
  },
  "dependencies": {
    "@aws-sdk/client-ec2": "^3.0.0",
    "@aws-sdk/client-ecs": "^3.0.0",
    "discord-interactions": "^3.4.0",
    "hono": "^4.0.0"
  },
  "devDependencies": {
    "@types/aws-lambda": "^8.10.0",
    "@types/node": "^22.0.0",
    "esbuild": "^0.20.0",
    "ts-node": "^10.9.0",
    "typescript": "^5.0.0"
  }
}
```

- [ ] **Step 2: Write tsconfig.json**

Write `lambda/launcher/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "lib": ["ES2022"],
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "outDir": "dist",
    "rootDir": "."
  },
  "include": ["src/**/*", "register-commands.ts"],
  "exclude": ["node_modules", "dist"]
}
```

- [ ] **Step 3: Install dependencies**

```bash
cd lambda/launcher && npm install
```

Expected: `node_modules/` created, no errors.

- [ ] **Step 4: Commit**

```bash
git add lambda/launcher/package.json lambda/launcher/tsconfig.json lambda/launcher/package-lock.json
git commit -m "feat(lambda): scaffold TypeScript project"
```

---

### Task 2: Implement games.ts — game control logic

**Files:**
- Create: `lambda/launcher/src/games.ts`

- [ ] **Step 1: Write games.ts**

Write `lambda/launcher/src/games.ts`:

```typescript
import { ECSClient, UpdateServiceCommand, ListTasksCommand, DescribeTasksCommand } from "@aws-sdk/client-ecs";
import { EC2Client, DescribeNetworkInterfacesCommand } from "@aws-sdk/client-ec2";

const REGION = process.env.AWS_REGION ?? "ca-central-1";
const CLUSTER = process.env.ECS_CLUSTER ?? "";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";
const MAX_POLLS = 10;
const POLL_INTERVAL_MS = 5000;

const ecs = new ECSClient({ region: REGION });
const ec2 = new EC2Client({ region: REGION });

export interface GameConfig {
  serviceName: string;
  sidecarPort: number;
}

export interface GameState {
  status: "offline" | "starting" | "online";
  publicIp?: string;
  players: number;
  ready: boolean;
  configUrl?: string;
}

export function getGames(): Record<string, GameConfig> {
  return JSON.parse(process.env.GAMES ?? "{}");
}

async function setDesiredCount(serviceName: string, count: number): Promise<void> {
  await ecs.send(new UpdateServiceCommand({
    cluster: CLUSTER,
    service: serviceName,
    desiredCount: count,
  }));
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

export async function getGameState(config: GameConfig): Promise<GameState> {
  try {
    const listRes = await ecs.send(new ListTasksCommand({ cluster: CLUSTER, serviceName: config.serviceName }));
    const taskArn = listRes.taskArns?.[0];
    if (!taskArn) return { status: "offline", players: 0, ready: false };

    const descRes = await ecs.send(new DescribeTasksCommand({ cluster: CLUSTER, tasks: [taskArn] }));
    const task = descRes.tasks?.[0];
    const eniId = task?.attachments?.[0]?.details?.find(d => d.name === "networkInterfaceId")?.value;
    if (!eniId) return { status: "starting", players: 0, ready: false };

    const eniRes = await ec2.send(new DescribeNetworkInterfacesCommand({ NetworkInterfaceIds: [eniId] }));
    const publicIp = eniRes.NetworkInterfaces?.[0]?.Association?.PublicIp;
    if (!publicIp) return { status: "starting", players: 0, ready: false };

    const sidecar = await getSidecarStatus(publicIp, config.sidecarPort);
    if (!sidecar) return { status: "starting", publicIp, players: 0, ready: false };

    const running = Boolean(sidecar.running);
    const ready = Boolean(sidecar.ready);
    const players = Number(sidecar.players ?? 0);

    return {
      status: running && ready ? "online" : "starting",
      publicIp,
      players,
      ready,
    };
  } catch {
    return { status: "offline", players: 0, ready: false };
  }
}

async function waitForState(config: GameConfig, desired: "online" | "offline"): Promise<GameState> {
  let state = await getGameState(config);
  for (let i = 0; i < MAX_POLLS; i++) {
    if (state.status === desired) return state;
    await new Promise(r => setTimeout(r, POLL_INTERVAL_MS));
    state = await getGameState(config);
  }
  return state;
}

export async function stopGame(config: GameConfig): Promise<GameState> {
  await setDesiredCount(config.serviceName, 0);
  return waitForState(config, "offline");
}

async function restartWithConfig(ip: string, port: number, configUrl: string): Promise<void> {
  await fetch(`http://${ip}:${port}/restart`, {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${SIDECAR_TOKEN}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ config_url: configUrl }),
    signal: AbortSignal.timeout(10000),
  });
}

export async function startGame(config: GameConfig, configUrl?: string): Promise<GameState> {
  await stopGame(config);
  await setDesiredCount(config.serviceName, 1);
  let state = await waitForState(config, "online");

  if (configUrl && state.status === "online" && state.publicIp) {
    await restartWithConfig(state.publicIp, config.sidecarPort, configUrl);
    state = await waitForState(config, "online");
    state.configUrl = configUrl;
  }

  return state;
}
```

- [ ] **Step 2: Verify it type-checks**

```bash
cd lambda/launcher && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add lambda/launcher/src/games.ts
git commit -m "feat(lambda): add game control logic in TypeScript"
```

---

### Task 3: Implement ui.ts — web UI

**Files:**
- Create: `lambda/launcher/src/ui.ts`

- [ ] **Step 1: Write ui.ts**

Write `lambda/launcher/src/ui.ts`:

```typescript
const GAMES = ["xonotic", "qssm", "q2repro"];

export function renderUi(): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>insta-game</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { background: #111; color: #eee; font-family: monospace; padding: 2rem; }
    h1 { margin-bottom: 2rem; font-size: 1.4rem; }
    #auth { max-width: 400px; }
    #auth input { width: 100%; padding: .5rem; background: #222; color: #eee; border: 1px solid #444; margin-bottom: .5rem; }
    #auth button { padding: .5rem 1rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; }
    .games { display: flex; gap: 1rem; flex-wrap: wrap; }
    .game { background: #1a1a1a; border: 1px solid #333; padding: 1rem; min-width: 200px; }
    .game h2 { margin-bottom: .75rem; font-size: 1rem; }
    .status { margin-bottom: .75rem; font-size: .85rem; color: #aaa; }
    .status.online { color: #4f4; }
    .status.starting { color: #fa4; }
    .actions { display: flex; gap: .5rem; }
    .actions button { padding: .4rem .8rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; }
    .actions button:hover { background: #444; }
    #error { color: #f44; margin-top: 1rem; display: none; }
  </style>
</head>
<body>
  <h1>insta-game</h1>

  <div id="auth">
    <input type="password" id="passphrase" placeholder="passphrase" />
    <button onclick="authenticate()">unlock</button>
  </div>

  <div id="panel" style="display:none">
    <div class="games">
      ${GAMES.map(g => `
      <div class="game" id="game-${g}">
        <h2>${g}</h2>
        <div class="status" id="status-${g}">loading...</div>
        <div class="actions">
          <button onclick="action('${g}', 'start')">start</button>
          <button onclick="action('${g}', 'stop')">stop</button>
          <button onclick="refresh('${g}')">status</button>
        </div>
      </div>`).join("")}
    </div>
    <div id="error"></div>
  </div>

  <script>
    const SESSION_KEY = "insta-game-passphrase";
    let passphrase = sessionStorage.getItem(SESSION_KEY) ?? "";

    function authenticate() {
      passphrase = document.getElementById("passphrase").value;
      sessionStorage.setItem(SESSION_KEY, passphrase);
      showPanel();
    }

    function showPanel() {
      if (!passphrase) return;
      document.getElementById("auth").style.display = "none";
      document.getElementById("panel").style.display = "";
      ${GAMES.map(g => `refresh("${g}");`).join(" ")}
    }

    function logout() {
      sessionStorage.removeItem(SESSION_KEY);
      passphrase = "";
      document.getElementById("auth").style.display = "";
      document.getElementById("panel").style.display = "none";
    }

    async function refresh(game) {
      const el = document.getElementById("status-" + game);
      try {
        const res = await fetch("/api?game=" + game, { headers: { "X-API-Token": passphrase } });
        if (res.status === 401) { logout(); return; }
        const data = await res.json();
        el.className = "status " + data.status;
        el.textContent = data.status + (data.publicIp ? " — " + data.publicIp : "") + (data.players ? " (" + data.players + " players)" : "");
      } catch {
        el.textContent = "error";
      }
    }

    async function action(game, op) {
      const el = document.getElementById("status-" + game);
      el.className = "status starting";
      el.textContent = op === "start" ? "starting..." : "stopping...";
      try {
        const res = await fetch("/", {
          method: "POST",
          headers: { "X-Passphrase": passphrase, "Content-Type": "application/json" },
          body: JSON.stringify({ game, operation: op }),
        });
        if (res.status === 401) { logout(); return; }
        const data = await res.json();
        el.className = "status " + data.status;
        el.textContent = data.status + (data.publicIp ? " — " + data.publicIp : "");
      } catch {
        el.textContent = "error";
      }
    }

    // Poll all games every 10s
    setInterval(() => { if (passphrase) ${GAMES.map(g => `refresh("${g}")`).join(", ")}; }, 10000);

    // Auto-show panel if passphrase already in sessionStorage
    if (passphrase) showPanel();
  </script>
</body>
</html>`;
}
```

- [ ] **Step 2: Verify it type-checks**

```bash
cd lambda/launcher && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add lambda/launcher/src/ui.ts
git commit -m "feat(lambda): add web UI template"
```

---

### Task 4: Implement discord.ts — interaction handler

**Files:**
- Create: `lambda/launcher/src/discord.ts`

- [ ] **Step 1: Write discord.ts**

Write `lambda/launcher/src/discord.ts`:

```typescript
import { verifyKey, InteractionType, InteractionResponseType } from "discord-interactions";
import type { Context } from "hono";
import { getGames, getGameState, startGame, stopGame } from "./games.js";

const DISCORD_PUBLIC_KEY = process.env.DISCORD_PUBLIC_KEY ?? "";
const DISCORD_BOT_TOKEN = process.env.DISCORD_BOT_TOKEN ?? "";
const DISCORD_APP_ID = process.env.DISCORD_APP_ID ?? "";

export async function discordHandler(c: Context): Promise<Response> {
  const signature = c.req.header("x-signature-ed25519") ?? "";
  const timestamp = c.req.header("x-signature-timestamp") ?? "";
  const rawBody = await c.req.text();

  const isValid = verifyKey(rawBody, signature, timestamp, DISCORD_PUBLIC_KEY);
  if (!isValid) return c.text("invalid signature", 401);

  const interaction = JSON.parse(rawBody) as {
    type: number;
    id: string;
    token: string;
    data?: { name: string; options?: Array<{ name: string; value: string }> };
  };

  // PING
  if (interaction.type === InteractionType.PING) {
    return c.json({ type: InteractionResponseType.PONG });
  }

  // Slash command
  if (interaction.type === InteractionType.APPLICATION_COMMAND) {
    const name = interaction.data?.name ?? "";
    const gameName = interaction.data?.options?.find(o => o.name === "game")?.value ?? "";
    const games = getGames();
    const config = games[gameName];

    if (!config) {
      return c.json({
        type: InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
        data: { content: `Unknown game: \`${gameName}\`` },
      });
    }

    if (name === "status") {
      const state = await getGameState(config);
      return c.json({
        type: InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
        data: { content: formatState(gameName, state) },
      });
    }

    if (name === "stop") {
      const state = await stopGame(config);
      return c.json({
        type: InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
        data: { content: formatState(gameName, state) },
      });
    }

    if (name === "start") {
      // Defer immediately — start can take up to ~50s
      void sendFollowup(interaction.token, gameName, config);
      return c.json({ type: InteractionResponseType.DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE });
    }

    return c.json({
      type: InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
      data: { content: `Unknown command: \`${name}\`` },
    });
  }

  return c.text("unhandled interaction type", 400);
}

async function sendFollowup(
  interactionToken: string,
  gameName: string,
  config: Parameters<typeof startGame>[0]
): Promise<void> {
  const state = await startGame(config);
  const content = formatState(gameName, state);

  await fetch(
    `https://discord.com/api/v10/webhooks/${DISCORD_APP_ID}/${interactionToken}/messages/@original`,
    {
      method: "PATCH",
      headers: {
        "Authorization": `Bot ${DISCORD_BOT_TOKEN}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ content }),
    }
  );
}

function formatState(gameName: string, state: { status: string; publicIp?: string; players: number }): string {
  const parts = [`**${gameName}** — ${state.status}`];
  if (state.publicIp) parts.push(`\`${state.publicIp}\``);
  if (state.players > 0) parts.push(`${state.players} player(s)`);
  return parts.join(" — ");
}
```

- [ ] **Step 2: Verify it type-checks**

```bash
cd lambda/launcher && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add lambda/launcher/src/discord.ts
git commit -m "feat(lambda): add Discord interaction handler"
```

---

### Task 5: Implement index.ts — Hono app entry point

**Files:**
- Create: `lambda/launcher/src/index.ts`

- [ ] **Step 1: Write index.ts**

Write `lambda/launcher/src/index.ts`:

```typescript
import { Hono } from "hono";
import { handle } from "hono/aws-lambda";
import { getGames, getGameState, startGame, stopGame } from "./games.js";
import { discordHandler } from "./discord.js";
import { renderUi } from "./ui.js";

const WEB_UI_PASSPHRASE = process.env.WEB_UI_PASSPHRASE ?? "";
const API_TOKEN = process.env.API_TOKEN ?? "";

const app = new Hono();

// Web UI
app.get("/", c => c.html(renderUi()));

// Web UI form submission
app.post("/", async c => {
  const passphrase = c.req.header("x-passphrase") ?? "";
  if (passphrase !== WEB_UI_PASSPHRASE) return c.json({ error: "unauthorized" }, 401);

  const body = await c.req.json<{ game: string; operation: string }>();
  const games = getGames();
  const config = games[body.game];
  if (!config) return c.json({ error: `unknown game: ${body.game}` }, 400);

  if (body.operation === "start") {
    const state = await startGame(config);
    return c.json(state);
  }
  if (body.operation === "stop") {
    const state = await stopGame(config);
    return c.json(state);
  }
  const state = await getGameState(config);
  return c.json(state);
});

// JSON API
app.get("/api", async c => {
  const token = c.req.header("x-api-token") ?? "";
  if (token !== API_TOKEN) return c.json({ error: "unauthorized" }, 401);

  const game = c.req.query("game") ?? "";
  const operation = c.req.query("operation");
  const games = getGames();
  const config = games[game];
  if (!config) return c.json({ error: `unknown game: ${game}` }, 400);

  if (operation === "start") return c.json(await startGame(config));
  if (operation === "stop") return c.json(await stopGame(config));
  return c.json(await getGameState(config));
});

// Discord webhook
app.post("/discord", discordHandler);

export const handler = handle(app);
```

- [ ] **Step 2: Build and verify**

```bash
cd lambda/launcher && npm run build
```

Expected: `dist/index.js` created, no errors.

- [ ] **Step 3: Commit**

```bash
git add lambda/launcher/src/index.ts lambda/launcher/dist/
git commit -m "feat(lambda): add Hono app entry point"
```

---

### Task 6: Implement register-commands.ts

**Files:**
- Create: `lambda/launcher/register-commands.ts`

- [ ] **Step 1: Write register-commands.ts**

Write `lambda/launcher/register-commands.ts`:

```typescript
const APP_ID = process.env.DISCORD_APP_ID ?? "";
const BOT_TOKEN = process.env.DISCORD_BOT_TOKEN ?? "";

if (!APP_ID || !BOT_TOKEN) {
  console.error("DISCORD_APP_ID and DISCORD_BOT_TOKEN must be set");
  process.exit(1);
}

const GAME_CHOICES = [
  { name: "Xonotic", value: "xonotic" },
  { name: "QSS-M (Quake 1)", value: "qssm" },
  { name: "q2repro (Quake 2)", value: "q2repro" },
];

const commands = [
  {
    name: "start",
    description: "Start a game server",
    options: [{
      name: "game",
      description: "Which game to start",
      type: 3, // STRING
      required: true,
      choices: GAME_CHOICES,
    }],
  },
  {
    name: "stop",
    description: "Stop a game server",
    options: [{
      name: "game",
      description: "Which game to stop",
      type: 3,
      required: true,
      choices: GAME_CHOICES,
    }],
  },
  {
    name: "status",
    description: "Get the status of a game server",
    options: [{
      name: "game",
      description: "Which game to check",
      type: 3,
      required: true,
      choices: GAME_CHOICES,
    }],
  },
];

async function main() {
  const res = await fetch(`https://discord.com/api/v10/applications/${APP_ID}/commands`, {
    method: "PUT",
    headers: {
      "Authorization": `Bot ${BOT_TOKEN}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(commands),
  });

  if (!res.ok) {
    const err = await res.text();
    console.error("Failed to register commands:", err);
    process.exit(1);
  }

  const data = await res.json();
  console.log("Registered commands:", JSON.stringify(data, null, 2));
}

main();
```

- [ ] **Step 2: Verify it type-checks**

```bash
cd lambda/launcher && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Document usage in README**

In `README.md`, add a section under **Infrastructure Workflow**:

```markdown
### Registering Discord slash commands

Run once after setting up the Discord application:

```sh
cd lambda/launcher
DISCORD_APP_ID=<app-id> DISCORD_BOT_TOKEN=<bot-token> npx ts-node register-commands.ts
```

This registers `/start`, `/stop`, and `/status` globally. Safe to re-run.
```

- [ ] **Step 4: Commit**

```bash
git add lambda/launcher/register-commands.ts README.md
git commit -m "feat(lambda): add Discord command registration script"
```

---

### Task 7: Update Pulumi to deploy the new Lambda

**Files:**
- Modify: `pulumi/__main__.py`

- [ ] **Step 1: Update the launcher resource in pulumi/__main__.py**

Replace the existing `launcher = aws.lambda_.Function(...)` block with:

```python
launcher = aws.lambda_.Function(
    "launcher",
    name=regional_name("launcher"),
    runtime="nodejs22.x",
    handler="index.handler",
    timeout=120,
    role=lambda_role.arn,
    code=pulumi.FileArchive("../lambda/launcher/dist"),
    environment=aws.lambda_.FunctionEnvironmentArgs(
        variables=pulumi.Output.all(
            sidecar_token,
            xonotic.service_name,
            cluster.name,
            qssm.service_name,
            q2repro.service_name,
            web_ui_passphrase,
            api_token,
            discord_public_key,
            discord_bot_token,
            discord_app_id,
        ).apply(
            lambda args: {
                "SIDECAR_TOKEN": args[0],
                "ECS_CLUSTER": args[2],
                "WEB_UI_PASSPHRASE": args[5],
                "API_TOKEN": args[6],
                "DISCORD_PUBLIC_KEY": args[7],
                "DISCORD_BOT_TOKEN": args[8],
                "DISCORD_APP_ID": args[9],
                "GAMES": json.dumps(
                    {
                        "xonotic": {
                            "serviceName": args[1],
                            "sidecarPort": 5001,
                        },
                        "qssm": {
                            "serviceName": args[3],
                            "sidecarPort": 5001,
                        },
                        "q2repro": {
                            "serviceName": args[4],
                            "sidecarPort": 5001,
                        },
                    }
                ),
            }
        ),
    ),
)
```

Note the `GAMES` keys change from `service_name`/`sidecar_port` to `serviceName`/`sidecarPort` to match TypeScript camelCase conventions.

- [ ] **Step 2: Add new config variables at the top of __main__.py alongside existing ones**

Find the existing config block (where `sidecar_token` is read) and add:

```python
web_ui_passphrase = config.require_secret("webUiPassphrase")
api_token = config.require_secret("apiToken")
discord_public_key = config.require_secret("discordPublicKey")
discord_bot_token = config.require_secret("discordBotToken")
discord_app_id = config.get("discordAppId") or ""
```

- [ ] **Step 3: Run ruff**

```bash
cd pulumi && uv run ruff check . && uv run ruff format --check .
```

Expected: no errors.

- [ ] **Step 4: Set the new Pulumi config values**

```bash
cd pulumi
uv run pulumi config set --secret webUiPassphrase <your-passphrase>
uv run pulumi config set --secret apiToken <your-api-token>
uv run pulumi config set --secret discordPublicKey <from-discord-developer-portal>
uv run pulumi config set --secret discordBotToken <from-discord-developer-portal>
uv run pulumi config set discordAppId <from-discord-developer-portal>
```

- [ ] **Step 5: Build the Lambda bundle**

```bash
cd lambda/launcher && npm run build
```

Expected: `dist/index.js` updated, no errors.

- [ ] **Step 6: Run pulumi preview**

```bash
cd pulumi && uv run pulumi preview
```

Expected: launcher function updated (runtime python3.12 → nodejs22.x, handler, code, env vars). No unexpected replacements.

- [ ] **Step 7: Commit**

```bash
git add pulumi/__main__.py pulumi/Pulumi.yaml
git commit -m "infra: update Lambda to Node 22 TypeScript runtime with new env vars"
```

---

### Task 8: Delete launcher.py and add .gitignore for dist/

**Files:**
- Delete: `lambda/launcher/launcher.py`
- Modify: `.gitignore`

- [ ] **Step 1: Delete the old Python handler**

```bash
rm lambda/launcher/launcher.py
```

- [ ] **Step 2: Add dist/ and node_modules/ to .gitignore**

Add to `.gitignore`:

```
lambda/launcher/node_modules/
lambda/launcher/dist/
```

- [ ] **Step 3: Commit**

```bash
git add -A lambda/launcher/launcher.py .gitignore
git commit -m "chore: remove Python Lambda, gitignore dist and node_modules"
```

---

### Task 9: Deploy and smoke test

- [ ] **Step 1: Build the Lambda bundle**

```bash
cd lambda/launcher && npm run build
```

- [ ] **Step 2: Deploy**

```bash
cd pulumi && uv run pulumi up
```

Confirm preview, select yes.

- [ ] **Step 3: Get the function URL**

```bash
uv run pulumi stack output prod_url
```

- [ ] **Step 4: Smoke test the web UI**

Open `<prod_url>` in a browser. Should show the passphrase input. Enter the passphrase — panel should appear with three game cards showing status.

- [ ] **Step 5: Smoke test the JSON API**

```bash
curl -H "X-API-Token: <api-token>" "<prod_url>/api?game=xonotic"
```

Expected: `{"status":"offline","players":0,"ready":false}`

- [ ] **Step 6: Smoke test Discord**

In Discord, run `/status xonotic`. Expected: bot responds with current server status.

- [ ] **Step 7: Commit any final fixes, then done**
