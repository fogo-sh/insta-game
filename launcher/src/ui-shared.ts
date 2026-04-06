import type { CachedGameState } from "./backend.js";

export const SESSION_KEY = "insta-game-passphrase";

export function rowHeaderId(game: string): string {
  return `row-header-${game}`;
}

export function rowBodyId(game: string): string {
  return `row-body-${game}`;
}

export function expandButtonId(game: string): string {
  return `expand-btn-${game}`;
}

export function logSectionId(game: string): string {
  return `log-section-${game}`;
}

export function logPanelId(game: string): string {
  return `log-sse-${game}`;
}

function escapeHtml(text: string): string {
  return text
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function statusDot(status: string): string {
  if (status === "online") return "🟢";
  if (status === "starting") return "🟡";
  return "⚫";
}

export function renderRowHeaderContent(label: string, game: string, state: CachedGameState): string {
  const meta: string[] = [];
  if (state.status === "online" && state.hostname) meta.push(`<span>${escapeHtml(state.hostname)}</span>`);
  if (state.status === "online" && state.map) meta.push(`<span>${escapeHtml(state.map)}</span>`);
  if (state.status === "online") meta.push(`<span>${state.players} player${state.players !== 1 ? "s" : ""}</span>`);
  if (state.status !== "online") meta.push(`<span class="${state.status}">${state.status}</span>`);

  return [
    `<span class="status-dot">${statusDot(state.status)}</span>`,
    `<span class="game-name">${escapeHtml(label)}</span>`,
    `<span class="row-meta">${meta.join("")}</span>`,
    `<button class="expand-btn" id="${expandButtonId(game)}">[expand ▼]</button>`,
  ].join("");
}
