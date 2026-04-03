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

async function restartWithConfig(port: number, configUrl: string): Promise<void> {
  await fetch(`http://localhost:${port}/restart`, {
    method: "POST",
    headers: { "Authorization": `Bearer ${SIDECAR_TOKEN}`, "Content-Type": "application/json" },
    body: JSON.stringify({ config_url: configUrl }),
    signal: AbortSignal.timeout(10000),
  });
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

  async startGame(config: GameConfig, configUrl?: string): Promise<GameState> {
    const c = config as DockerGameConfig;
    await dockerRequest("POST", `/containers/${encodeURIComponent(c.containerName)}/start`);
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
    // t=15 gives the container 15s to stop gracefully before SIGKILL
    await dockerRequest("POST", `/containers/${encodeURIComponent(c.containerName)}/stop?t=15`);
    return waitForState(this, config, "offline");
  }
}
