import { ECSClient, UpdateServiceCommand, ListTasksCommand, DescribeTasksCommand } from "@aws-sdk/client-ecs";
import { EC2Client, DescribeNetworkInterfacesCommand } from "@aws-sdk/client-ec2";
import type { Backend, GameConfig, GameState, CachedGameState } from "../backend.js";

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
        publicIp,
        players: Number(sidecar.players ?? 0),
        hostname: String(sidecar.hostname ?? ""),
        map: String(sidecar.map ?? ""),
        updatedAt: new Date(),
      };
    } catch {
      return offline;
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
