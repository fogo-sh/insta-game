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
