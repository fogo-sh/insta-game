import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const launcherRoot = path.resolve(__dirname, "..");
const repoRoot = path.resolve(launcherRoot, "..");
const dockerRoot = path.join(repoRoot, "docker-containers");
const distDir = path.join(launcherRoot, "dist");

function loadDefaultConfigText(gameDir, metadata) {
  const candidates = [
    metadata.defaultConfigFile,
    "server.cfg",
    "UnrealTournament.ini",
  ].filter(Boolean);

  for (const candidate of candidates) {
    const configPath = path.join(gameDir, candidate);
    try {
      return readFileSync(configPath, "utf8");
    } catch {
      continue;
    }
  }

  return "";
}

const catalog = {};
for (const entry of readdirSync(dockerRoot, { withFileTypes: true })) {
  if (!entry.isDirectory()) continue;
  const gameDir = path.join(dockerRoot, entry.name);
  const metadataPath = path.join(gameDir, "game.json");
  const dockerfilePath = path.join(gameDir, "Dockerfile");

  try {
    const metadata = JSON.parse(readFileSync(metadataPath, "utf8"));
    readFileSync(dockerfilePath, "utf8");
    catalog[metadata.id] = {
      displayName: metadata.displayName ?? metadata.id,
      clientDownloadUrl: metadata.clientDownloadUrl ?? null,
      configEditor: metadata.configEditor ?? null,
      defaultConfigText: loadDefaultConfigText(gameDir, metadata),
    };
  } catch {
    continue;
  }
}

mkdirSync(distDir, { recursive: true });
writeFileSync(path.join(distDir, "game-catalog.json"), JSON.stringify(catalog, null, 2));
