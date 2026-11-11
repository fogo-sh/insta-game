export type Desired = "running" | "stopped" | "deleted";
export type Phase =
  | "queued"
  | "preparing"
  | "starting"
  | "running"
  | "stopping"
  | "stopped"
  | "deleted"
  | "failed"
  | "unknown"
  | "unavailable"
  | "sleeping";
export type Action = "start" | "stop" | "remove";

export interface ServerStatus {
  desired: Desired;
  phase: Phase;
  pending?: boolean;
  action?: Action;
  error?: string;
  requestError?: string;
}

export type ControlValue = string | number | boolean | string[];
export type Render =
  | { mode: "toggle"; trueLines: string[]; falseLines: string[] }
  | { mode: "choice"; choices: Record<string, string[]> }
  | {
      mode: "sequence";
      emptyLines?: string[];
      beforeLines?: string[];
      itemLines: string[];
      afterLines?: string[];
    }
  | { mode: "template" | "join"; lines: string[]; separator?: string };

interface ControlBase {
  id: string;
  label: string;
  render: Render;
}

type Option = { value: string; label: string };
export type ConfigControl = ControlBase &
  (
    | { type: "text"; defaultValue: string }
    | { type: "number"; defaultValue: number; min?: number; max?: number }
    | { type: "checkbox"; defaultValue: boolean }
    | { type: "radio"; defaultValue: string; options: Option[] }
    | { type: "multiselect"; defaultValue: string[]; options: Option[] }
  );

export interface Game {
  id: string;
  displayName: string;
  gamePort: number;
  defaultConfig: string;
  configEditor?: {
    controls: ConfigControl[];
    managedBlockStart: string;
    managedBlockEnd: string;
  };
}

export interface Instance extends ServerStatus {
  id: string;
  name: string;
  game: Game;
  provider: string;
  endpoints: { port: number; address: string; protocol: string }[] | null;
  players: number | null;
  map: string;
  config: string;
  createdAt: string;
  idleMinutes: number;
  maxMinutes: number;
  stopReason?: string;
}

export interface LaunchServer extends ServerStatus {
  id: string;
  name: string;
  game: string;
  hostId?: string;
  address?: string;
  players?: number | null;
  observedAt?: string;
}

export interface Host {
  id: string;
  name: string;
  state: string;
  error?: string;
}

export interface Choice {
  id: string;
  name: string;
}
export interface Preset extends Choice {
  game?: string;
  hostId?: string;
}
export interface Listing {
  host?: string;
  hosts?: Host[];
  games: LaunchServer[];
  choices?: Choice[];
  presets?: Preset[];
}
