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
