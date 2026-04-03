import type { Backend } from "../backend.js";
import { EcsBackend } from "./ecs.js";
import { DockerBackend } from "./docker.js";

export function createBackend(): Backend {
  const backend = process.env.BACKEND ?? "ecs";
  if (backend === "docker") return new DockerBackend();
  return new EcsBackend();
}
