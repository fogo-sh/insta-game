export interface GameConfig {
  sidecarPort: number;
  // Backend-specific fields (e.g. serviceName, containerName) are opaque to callers.
  [key: string]: unknown;
}

export interface GameState {
  status: "offline" | "starting" | "online";
  publicIp?: string;
  players: number;
  ready: boolean;
  configUrl?: string;
}

// Extended state used by the public cache — includes sidecar-reported fields
export interface CachedGameState {
  status: "offline" | "starting" | "online";
  players: number;
  hostname: string;
  map: string;
  updatedAt: Date;
}

export interface Backend {
  getGames(): Record<string, GameConfig>;
  getGameState(config: GameConfig): Promise<GameState>;
  getCachedState(config: GameConfig): Promise<CachedGameState>;
  startGame(config: GameConfig, configUrl?: string): Promise<GameState>;
  stopGame(config: GameConfig): Promise<GameState>;
}
