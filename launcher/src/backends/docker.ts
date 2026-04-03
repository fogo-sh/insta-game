import type { Backend, GameConfig, GameState } from "../backend.js";

export interface DockerGameConfig extends GameConfig {
  containerName: string;
}

export class DockerBackend implements Backend {
  getGames(): Record<string, DockerGameConfig> {
    return JSON.parse(process.env.GAMES ?? "{}");
  }

  async getGameState(_config: GameConfig): Promise<GameState> {
    return { status: "offline", players: 0, ready: false };
  }

  async startGame(_config: GameConfig): Promise<GameState> {
    return { status: "offline", players: 0, ready: false };
  }

  async stopGame(_config: GameConfig): Promise<GameState> {
    return { status: "offline", players: 0, ready: false };
  }
}
