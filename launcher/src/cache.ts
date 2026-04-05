import type { Backend, CachedGameState } from "./backend.js";
import { log } from "./logger.js";

const POLL_INTERVAL_MS = 30_000;

export class GameCache {
  private cache = new Map<string, CachedGameState>();
  private timer: ReturnType<typeof setInterval> | null = null;

  constructor(private readonly backend: Backend) {}

  start(): void {
    // Poll immediately on startup, then every 30s
    void this.pollAll();
    this.timer = setInterval(() => { void this.pollAll(); }, POLL_INTERVAL_MS);
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer);
  }

  get(gameKey: string): CachedGameState | null {
    return this.cache.get(gameKey) ?? null;
  }

  set(gameKey: string, state: CachedGameState): void {
    this.cache.set(gameKey, state);
  }

  private async pollAll(): Promise<void> {
    const games = this.backend.getGames();
    await Promise.all(
      Object.entries(games).map(async ([key, config]) => {
        try {
          const state = await this.backend.getCachedState(config);
          this.cache.set(key, state);
        } catch (err) {
          log.error(`cache: poll failed for ${key}`, err);
          this.cache.set(key, {
            status: "offline",
            players: 0,
            hostname: "",
            map: "",
            updatedAt: new Date(),
          });
        }
      })
    );
  }
}
