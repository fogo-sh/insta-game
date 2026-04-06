import { readdirSync, readFileSync } from "fs";
import path from "path";

export interface PortBinding {
  hostIp?: string;
  hostPort: string;
}

export interface DockerGameDefinition {
  id: string;
  displayName: string;
  protocol: string;
  sidecarPort: number;
  gamePort?: number;
  ports?: Record<string, PortBinding>;
  gameCmd: string;
  gameArgs?: string;
  gameQuitCmd?: string;
  gameQuitTimeout?: number;
  configPath?: string;
  volumes?: string[];
  dataUrlEnv?: string;
  image?: string;
  containerName?: string;
  clientDownloadUrl?: string;
}

export function loadDockerGameDefinitions(repoRoot: string): DockerGameDefinition[] {
  const dockerRoot = path.join(repoRoot, "docker-containers");
  const entries = readdirSync(dockerRoot, { withFileTypes: true })
    .filter(entry => entry.isDirectory())
    .map(entry => entry.name)
    .sort();

  const definitions: DockerGameDefinition[] = [];
  for (const entry of entries) {
    const gameDir = path.join(dockerRoot, entry);
    const dockerfilePath = path.join(gameDir, "Dockerfile");
    const metadataPath = path.join(gameDir, "game.json");

    try {
      const metadata = JSON.parse(readFileSync(metadataPath, "utf8")) as DockerGameDefinition;
      readFileSync(dockerfilePath, "utf8");
      definitions.push(metadata);
    } catch {
      continue;
    }
  }

  return definitions;
}

export function defaultRepoRoot(fromDir: string): string {
  return path.resolve(fromDir, "..");
}
