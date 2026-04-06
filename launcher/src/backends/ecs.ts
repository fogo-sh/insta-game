import { ECSClient, UpdateServiceCommand, ListTasksCommand, DescribeTasksCommand } from "@aws-sdk/client-ecs";
import { EC2Client, DescribeNetworkInterfacesCommand } from "@aws-sdk/client-ec2";
import type { Backend, GameConfig, GameState, CachedGameState } from "../backend.js";

const REGION = process.env.AWS_REGION ?? "ca-central-1";
const CLUSTER = process.env.ECS_CLUSTER ?? "";
const SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";

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
      return { status: running ? "online" : "starting", publicIp, players, ready };
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
      return {
        status: running ? "online" : "starting",
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
    return { status: "offline", players: 0, ready: false };
  }

  async startGame(config: GameConfig, configUrl?: string): Promise<GameState> {
    const c = config as EcsGameConfig;
    const current = await this.getGameState(config);
    if (current.status === "online" && !configUrl) return current;

    await setDesiredCount(c.serviceName, 1);

    if (configUrl && current.publicIp) {
      await restartWithConfig(current.publicIp, c.sidecarPort, configUrl);
      return { ...current, configUrl };
    }

    return { status: "starting", players: 0, ready: false };
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

async function restartWithConfig(ip: string, port: number, configUrl: string): Promise<void> {
  await fetch(`http://${ip}:${port}/restart`, {
    method: "POST",
    headers: { "Authorization": `Bearer ${SIDECAR_TOKEN}`, "Content-Type": "application/json" },
    body: JSON.stringify({ config_url: configUrl }),
    signal: AbortSignal.timeout(10000),
  });
}
