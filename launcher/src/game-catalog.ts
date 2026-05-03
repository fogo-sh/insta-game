import { readFileSync } from "fs";
import { join } from "path";
import type { ConfigEditorDefinition } from "./config-editor.js";

export interface GameCatalogEntry {
  displayName: string;
  clientDownloadUrl: string | null;
  configEditor: ConfigEditorDefinition | null;
  defaultConfigText: string;
}

let cachedCatalog: Record<string, GameCatalogEntry> | null = null;

export function loadGameCatalog(distDir: string): Record<string, GameCatalogEntry> {
  if (cachedCatalog) return cachedCatalog;
  try {
    const raw = readFileSync(join(distDir, "game-catalog.json"), "utf8");
    cachedCatalog = JSON.parse(raw) as Record<string, GameCatalogEntry>;
  } catch {
    cachedCatalog = {};
  }
  return cachedCatalog;
}
