import http from "http";
import type { Backend, GameConfig, GameState, CachedGameState } from "../backend.js";
import { loadDockerGameDefinitions } from "../game-definitions.js";
import type { PortBinding } from "../game-definitions.js";
import { log } from "../logger.js";

const SOCKET = process.env.DOCKER_SOCKET ?? "/var/run/docker.sock";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";
const SIDECAR_HOST = process.env.SIDECAR_HOST ?? "localhost";
const DATA_DIR = process.env.DATA_DIR ?? "/data";
// HOST_DATA_DIR is the repo root as seen by the Docker daemon — used for game container
// volume bind mounts. When running inside a container this differs from DATA_DIR.
const HOST_DATA_DIR = process.env.HOST_DATA_DIR ?? DATA_DIR;
const MAX_POLLS = 20;
const POLL_INTERVAL_MS = 3000;

const RCON_PASSWORD = process.env.RCON_PASSWORD ?? "";

export interface DockerGameConfig extends GameConfig {
  containerName: string;
  image: string;
  displayName: string;
  connectPort?: number;
  clientDownloadUrl?: string;
  // port bindings: key is containerPort (e.g. "26000/udp"), value is host binding
  ports?: Record<string, PortBinding>;
  environment?: Record<string, string>;
  volumes?: string[];  // host:container[:options]
}

// Low-level Docker API call over Unix socket — rejects on non-2xx responses
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
        const parsed = (() => { try { return text ? JSON.parse(text) : null; } catch { return text; } })();
        const status = res.statusCode ?? 0;
        if (status < 200 || status >= 300) {
          const message = (parsed as Record<string, unknown>)?.message ?? text;
          reject(new Error(`Docker API ${method} ${path} → ${status}: ${message}`));
        } else {
          resolve(parsed);
        }
      });
    });
    req.on("error", reject);
    if (bodyStr) req.write(bodyStr);
    req.end();
  });
}

// Pull an image — streams progress lines, resolves when done
function pullImage(image: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const req = http.request({
      socketPath: SOCKET,
      path: `/images/create?fromImage=${encodeURIComponent(image)}`,
      method: "POST",
    }, res => {
      // consume the stream (progress JSON lines) so the socket doesn't stall
      res.on("data", () => {});
      res.on("end", () => {
        const status = res.statusCode ?? 0;
        if (status < 200 || status >= 300) {
          reject(new Error(`Docker pull ${image} → ${status}`));
        } else {
          resolve();
        }
      });
    });
    req.on("error", reject);
    req.end();
  });
}

// Remove a container (must be stopped first, or use force)
async function removeContainer(name: string): Promise<void> {
  await dockerRequest("DELETE", `/containers/${encodeURIComponent(name)}?force=true`);
}

// Ensure the container exists with the correct config, recreating it if stale
async function ensureContainer(c: DockerGameConfig): Promise<void> {
  const existing = await inspectContainer(c.containerName);
  if (existing) {
    // Check if the image matches — if not, container is stale and needs recreating
    const existingImage = (existing.Config as Record<string, unknown>)?.Image as string ?? "";
    const expectedBinds = (c.volumes ?? []).map(v => {
      const [hostPath, ...rest] = v.split(":");
      const absHost = hostPath.startsWith("/") ? hostPath : `${HOST_DATA_DIR}/${hostPath}`;
      return [absHost, ...rest].join(":");
    });
    const existingBinds: string[] = ((existing.HostConfig as Record<string, unknown>)?.Binds as string[]) ?? [];
    const bindsMatch = expectedBinds.every(b => existingBinds.includes(b));
    const imageMatches = existingImage === c.image || existingImage.startsWith(c.image + "@");
    if (imageMatches && bindsMatch) return;
    log.info(`docker: removing stale container ${c.containerName} (image or mounts changed)`);
    await removeContainer(c.containerName);
  }

  log.info(`docker: image pull ${c.image}`);
  await pullImage(c.image);
  log.info(`docker: pulled ${c.image}`);

  // Build HostConfig port bindings: { "26000/udp": [{ HostIp, HostPort }] }
  const portBindings: Record<string, Array<{ HostIp: string; HostPort: string }>> = {};
  const exposedPorts: Record<string, Record<string, never>> = {};
  for (const [containerPort, binding] of Object.entries(c.ports ?? {})) {
    const key = containerPort.includes("/") ? containerPort : `${containerPort}/tcp`;
    portBindings[key] = [{ HostIp: binding.hostIp ?? "0.0.0.0", HostPort: binding.hostPort }];
    exposedPorts[key] = {};
  }

  // Sidecar port must always be exposed
  const sidecarKey = `${c.sidecarPort}/tcp`;
  if (!portBindings[sidecarKey]) {
    portBindings[sidecarKey] = [{ HostIp: "127.0.0.1", HostPort: String(c.sidecarPort) }];
    exposedPorts[sidecarKey] = {};
  }

  const binds = (c.volumes ?? []).map(v => {
    // Make relative host paths absolute using HOST_DATA_DIR (the host-side repo root,
    // which may differ from DATA_DIR when the launcher runs inside a container).
    const [hostPath, ...rest] = v.split(":");
    const absHost = hostPath.startsWith("/") ? hostPath : `${HOST_DATA_DIR}/${hostPath}`;
    return [absHost, ...rest].join(":");
  });

  const env = Object.entries(c.environment ?? {}).map(([k, v]) => `${k}=${v}`);

  log.info(`docker: creating container ${c.containerName}`);
  await dockerRequest("POST", `/containers/create?name=${encodeURIComponent(c.containerName)}`, {
    Image: c.image,
    Env: env,
    ExposedPorts: exposedPorts,
    HostConfig: {
      PortBindings: portBindings,
      Binds: binds,
      RestartPolicy: { Name: "no" },
    },
  });
  log.info(`docker: created container ${c.containerName}`);
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
    const res = await fetch(`http://${SIDECAR_HOST}:${port}/status`, { signal: AbortSignal.timeout(5000) });
    if (!res.ok) return null;
    return res.json() as Promise<Record<string, unknown>>;
  } catch {
    return null;
  }
}

async function waitForState(backend: DockerBackend, config: GameConfig, desired: "online" | "offline"): Promise<GameState> {
  const c = config as DockerGameConfig;
  let state = await backend.getGameState(config);
  for (let i = 0; i < MAX_POLLS; i++) {
    if (state.status === desired) return state;
    log.info(`docker: waiting for ${c.containerName} to be ${desired} (currently ${state.status}, poll ${i + 1}/${MAX_POLLS})`);
    await new Promise(r => setTimeout(r, POLL_INTERVAL_MS));
    state = await backend.getGameState(config);
  }
  if (state.status !== desired) log.warn(`docker: ${c.containerName} did not reach ${desired} after ${MAX_POLLS} polls (stuck at ${state.status})`);
  return state;
}

async function restartWithConfig(port: number, configUrl: string): Promise<void> {
  await fetch(`http://${SIDECAR_HOST}:${port}/restart`, {
    method: "POST",
    headers: { "Authorization": `Bearer ${SIDECAR_TOKEN}`, "Content-Type": "application/json" },
    body: JSON.stringify({ config_url: configUrl }),
    signal: AbortSignal.timeout(10000),
  });
}

export class DockerBackend implements Backend {
  getGames(): Record<string, DockerGameConfig> {
    const configs: Record<string, DockerGameConfig> = {};

    for (const definition of loadDockerGameDefinitions(DATA_DIR)) {
      const environment: Record<string, string> = {
        GAME_CMD: definition.gameCmd,
        GAME_ARGS: definition.gameArgs ?? "",
        GAME_QUIT_CMD: definition.gameQuitCmd ?? "quit",
        GAME_QUIT_TIMEOUT: String(definition.gameQuitTimeout ?? 15),
        GAME_PORT: String(definition.gamePort ?? 26000),
        TOKEN: SIDECAR_TOKEN,
      };

      if (definition.configPath) environment.CONFIG_PATH = definition.configPath;
      if (definition.dataUrlEnv) {
        const value = process.env[definition.dataUrlEnv];
        if (value) environment.DATA_URL = value;
      }
      if (RCON_PASSWORD) environment.RCON_PASSWORD = RCON_PASSWORD;

      configs[definition.id] = {
        containerName: definition.containerName ?? `insta-game-${definition.id}-1`,
        image: definition.image ?? `ghcr.io/fogo-sh/insta-game:${definition.id}`,
        displayName: definition.displayName,
        connectPort: definition.gamePort,
        clientDownloadUrl: definition.clientDownloadUrl,
        sidecarPort: definition.sidecarPort,
        ports: definition.ports,
        environment,
        volumes: definition.volumes,
      };
    }

    return configs;
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
      return { status: running && ready ? "online" : "starting", publicIp: "localhost", players, ready };
    } catch {
      return { status: "offline", players: 0, ready: false };
    }
  }

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

  async startGame(config: GameConfig, configUrl?: string): Promise<GameState> {
    const c = config as DockerGameConfig;
    log.info(`docker: starting container ${c.containerName}`);
    try {
      await ensureContainer(c);
      await dockerRequest("POST", `/containers/${encodeURIComponent(c.containerName)}/start`);
    } catch (err) {
      log.error(`docker: failed to start ${c.containerName}`, err);
      return { status: "offline", players: 0, ready: false };
    }
    let state = await waitForState(this, config, "online");
    if (configUrl && state.status === "online") {
      const inspect = await inspectContainer(c.containerName);
      const hostPort = inspect ? getHostPort(inspect, c.sidecarPort) : null;
      if (hostPort) {
        await restartWithConfig(hostPort, configUrl);
        state = await waitForState(this, config, "online");
        state.configUrl = configUrl;
      }
    }
    return state;
  }

  async stopGame(config: GameConfig): Promise<GameState> {
    const c = config as DockerGameConfig;
    log.info(`docker: stopping container ${c.containerName}`);
    try {
      // t=15 gives the container 15s to stop gracefully before SIGKILL
      await dockerRequest("POST", `/containers/${encodeURIComponent(c.containerName)}/stop?t=15`);
    } catch (err) {
      log.error(`docker: failed to stop ${c.containerName}`, err);
      return { status: "offline", players: 0, ready: false };
    }
    return waitForState(this, config, "offline");
  }
}
