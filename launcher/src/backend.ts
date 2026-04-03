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

export interface Backend {
  getGames(): Record<string, GameConfig>;
  getGameState(config: GameConfig): Promise<GameState>;
  startGame(config: GameConfig, configUrl?: string): Promise<GameState>;
  stopGame(config: GameConfig): Promise<GameState>;
}
