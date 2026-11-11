import type { Game, Instance } from "./types.ts";
import { element, required, errorMessage } from "./dom.ts";
import { serverState, stateLabel, serverActions } from "./status.ts";
import { settings } from "./settings.ts";
const controls = {
  provider: required<HTMLSelectElement>("#provider"),
  game: required<HTMLSelectElement>("#game"),
  config: required<HTMLTextAreaElement>("#config"),
  token: required<HTMLInputElement>("#token"),
  name: required<HTMLInputElement>("#name"),
  idle: required<HTMLInputElement>("#idle"),
  max: required<HTMLInputElement>("#max"),
  launch: required<HTMLButtonElement>("#launch"),
  "create-form": required<HTMLFormElement>("#create-form"),
  "login-form": required<HTMLFormElement>("#login-form"),
  "create-dialog": required<HTMLDialogElement>("#create-dialog"),
  "server-template": required<HTMLTemplateElement>("#server-template"),
};
const $ = (id: string) => required<HTMLElement>(`#${id}`);
let token = sessionStorage.getItem("insta-game-token") || "";
let games: Game[] = [];
let instances: Instance[] = [];
let logAbort: AbortController | null = null;
let selectedID: string | null = null;
let createID = "";
function stopLogs() {
  logAbort?.abort();
  logAbort = null;
}
async function api<T = unknown>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch("/api" + path, {
    ...options,
    headers: {
      Authorization: "Bearer " + token,
      "Content-Type": "application/json",
    },
  });
  if (response.status === 401) {
    lock();
    throw new Error("Invalid controller token.");
  }
  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error || "Request failed");
  }
  return response.json();
}
function message(id: string, text: string) {
  $(id).textContent = text;
  $(id).hidden = !text;
}
function lock() {
  token = "";
  sessionStorage.removeItem("insta-game-token");
  $("login").hidden = false;
  $("dashboard").hidden = true;
  $("detail").hidden = true;
  $("signout").hidden = true;
  stopLogs();
}
function connectAddress(i: Instance | undefined) {
  if (!i || serverState(i) !== "running" || i.players == null) return "";
  return i.endpoints?.find((e) => e.port === i.game.gamePort)?.address || "";
}
function statusLabel(i: Instance) {
  return stateLabel(serverState(i));
}
function renderList() {
  const list = $("servers");
  list.replaceChildren();
  $("summary").textContent = `${instances.length} ${instances.length === 1 ? "server" : "servers"}`;
  if (!instances.length) {
    const empty = document.createElement("div");
    empty.className = "empty";
    empty.textContent = "No servers.";
    list.append(empty);
    return;
  }
  for (const i of [...instances].sort((a, b) => b.createdAt.localeCompare(a.createdAt))) {
    const row = controls["server-template"].content.cloneNode(true) as DocumentFragment;
    const name = required<HTMLAnchorElement>(".server-name", row);
    name.textContent = i.name;
    name.href = "#server/" + i.id;
    required(".server-meta", row).textContent = i.game.displayName + " / " + i.provider;
    const state = required(".phase", row);
    state.textContent = statusLabel(i);
    state.dataset.phase = serverState(i);
    required("code", row).textContent = connectAddress(i) || "Address pending";
    required(".players", row).textContent =
      (i.players === null ? "Players unknown" : `${i.players} players`) +
      (i.map ? " · " + i.map : "");
    list.append(row);
  }
}
function runtimeNotice(i: Instance) {
  return (
    i.stopReason ||
    `idle timeout: ${i.idleMinutes ? i.idleMinutes + " min" : "off"}; runtime limit: ${i.maxMinutes ? i.maxMinutes + " min" : "off"}`
  );
}
function renderActions(i: Instance, actions: HTMLElement) {
  actions.replaceChildren();
  for (const { label, action, disabled } of serverActions(i)) {
    const button = element("button", label, action === "remove" ? "danger quiet" : "");
    button.disabled = Boolean(disabled);
    button.dataset.action = action;
    button.onclick = async () => {
      if (action === "remove" && !confirm(`Remove ${i.name} and its saved data?`)) return;
      try {
        button.disabled = true;
        await api("/instances/" + i.id + (action === "remove" ? "" : "/" + action), {
          method: action === "remove" ? "DELETE" : "POST",
        });
        await refresh(true);
      } catch (err) {
        message("detail-error", errorMessage(err));
        button.disabled = false;
      }
    };
    actions.append(button);
  }
  actions.dataset.state = JSON.stringify(serverActions(i));
}
function renderDetail(i: Instance) {
  const detail = $("detail");
  detail.replaceChildren();
  const back = element("a", "servers", "back");
  back.href = "#";
  detail.append(back);
  const heading = element("div", "", "detail-head");
  const title = element("div");
  title.append(element("p", i.game.displayName, "muted"), element("h2", i.name));
  const actions = element("div", "", "actions");
  renderActions(i, actions);
  heading.append(title, actions);
  detail.append(heading);
  const error = element("p", i.error || "", "error");
  error.id = "detail-error";
  error.setAttribute("role", "alert");
  detail.append(error);
  const facts = element("dl", "", "facts");
  for (const [name, value] of [
    ["status", statusLabel(i)],
    ["players / map", `${i.players ?? "Unknown"} / ${i.map || "-"}`],
    ["address", connectAddress(i) || "Pending"],
  ]) {
    const fact = element("div");
    const valueElement = element("dd", value);
    valueElement.id =
      name === "status" ? "fact-status" : name === "address" ? "fact-address" : "fact-players";
    fact.append(element("dt", name), valueElement);
    facts.append(fact);
  }
  detail.append(facts);
  const copy = element("button", "copy address");
  copy.id = "copy-address";
  copy.disabled = !connectAddress(i);
  copy.onclick = async () => {
    try {
      await navigator.clipboard.writeText(
        connectAddress(instances.find((server) => server.id === i.id)),
      );
      copy.textContent = "copied";
    } catch {
      message("detail-error", "Could not copy. Select the address to copy it manually.");
    }
  };
  detail.append(copy);

  const reason = element("p", runtimeNotice(i), "notice");
  reason.id = "stop-reason";
  detail.append(reason);
  const panel = element("section", "", "panel");
  panel.append(element("h2", "console"));
  const logs = element("pre", "Logs are not streaming.", "log");
  logs.id = "log";
  const open = element("button", "show logs", "quiet");
  open.onclick = () => streamLogs(i.id);
  const commandForm = element("form", "", "inline");
  const command = element("input");
  command.placeholder = "command";
  command.setAttribute("aria-label", "command");
  command.required = true;
  const send = element("button", "send");
  commandForm.append(command, send);
  commandForm.onsubmit = async (event) => {
    event.preventDefault();
    try {
      await api("/instances/" + i.id + "/console", {
        method: "POST",
        body: JSON.stringify({ command: command.value }),
      });
      command.value = "";
    } catch (err) {
      message("detail-error", errorMessage(err));
    }
  };
  panel.append(open, logs, commandForm);
  detail.append(panel);
  const config = element("details");
  config.append(element("summary", "config"), element("pre", i.config, "config-view"));
  detail.append(config);
}
function updateDetail(i: Instance) {
  if (!document.getElementById("fact-status")) return;
  $("fact-status").textContent = statusLabel(i);
  $("fact-players").textContent = `${i.players ?? "Unknown"} / ${i.map || "-"}`;
  $("fact-address").textContent = connectAddress(i) || "Pending";
  required<HTMLButtonElement>("#copy-address").disabled = !connectAddress(i);
  $("detail-error").textContent = i.error || "";
  const actions = required("#detail .actions");
  const expected = serverActions(i);
  if (JSON.stringify(expected) !== actions.dataset.state) {
    renderActions(i, actions);
  }
  $("stop-reason").textContent = runtimeNotice(i);
}
async function streamLogs(id: string) {
  stopLogs();
  const stream = new AbortController();
  logAbort = stream;
  try {
    const response = await fetch("/api/instances/" + id + "/logs", {
      headers: { Authorization: "Bearer " + token },
      signal: stream.signal,
    });
    if (!response.ok || !response.body) throw new Error("Could not open logs.");
    if (logAbort !== stream) return;
    const logs = required("#log");
    const reader = response.body.getReader(),
      decoder = new TextDecoder();
    let pending = "";
    logs.textContent = "";
    while (true) {
      const { value, done } = await reader.read();
      if (done || logAbort !== stream) break;
      pending += decoder.decode(value, { stream: true });
      const lines = pending.split("\n");
      pending = lines.pop() ?? "";
      for (const line of lines)
        if (line.startsWith("data: ")) {
          logs.textContent += line.slice(6) + "\n";
        }
      logs.textContent = logs.textContent.slice(-100000);
      logs.scrollTop = logs.scrollHeight;
    }
  } catch (err) {
    const logs = document.getElementById("log");
    if (!(err instanceof DOMException && err.name === "AbortError") && logAbort === stream && logs)
      logs.textContent = errorMessage(err);
  } finally {
    if (logAbort === stream) logAbort = null;
  }
}
function route(force = false) {
  const id = location.hash.startsWith("#server/") ? location.hash.slice(8) : null;
  if (!token) return;
  $("dashboard").hidden = Boolean(id);
  $("detail").hidden = !id;
  if (id) {
    const i = instances.find((i) => i.id === id);
    if (i) {
      if (id !== selectedID || force || !document.getElementById("fact-status")) {
        stopLogs();
        renderDetail(i);
      }
    } else {
      stopLogs();
      $("detail").replaceChildren(element("p", "Server not found."));
    }
  } else if (selectedID) stopLogs();
  selectedID = id;
}
async function refresh(force = false) {
  if (!token) return;
  try {
    instances = await api<Instance[]>("/instances");
    renderList();
    route(force);
    $("refresh-status").textContent = "Updated " + new Date().toLocaleTimeString();
    message("error", "");
    if (selectedID) {
      const current = instances.find((i) => i.id === selectedID);
      if (current) updateDetail(current);
    }
  } catch (err) {
    message("error", errorMessage(err));
    $("refresh-status").textContent = "Connection interrupted";
  }
}
async function unlock() {
  games = await api<Game[]>("/catalog");
  const targets = await api<{ providers: string[]; default: string }>("/providers");
  controls["provider"].replaceChildren(
    ...targets.providers.map((name) => {
      const option = element("option", name);
      option.value = name;
      return option;
    }),
  );
  controls["provider"].value = targets.default;
  controls["game"].replaceChildren(
    ...games.map((g) => {
      const o = element("option", g.displayName);
      o.value = g.id;
      return o;
    }),
  );
  $("login").hidden = true;
  $("dashboard").hidden = false;
  $("signout").hidden = false;
  sessionStorage.setItem("insta-game-token", token);
  await refresh(true);
}
controls["login-form"].onsubmit = async (event) => {
  event.preventDefault();
  token = controls["token"].value;
  try {
    await unlock();
    controls["token"].value = "";
  } catch (err) {
    message("login-error", errorMessage(err));
  }
};
$("signout").onclick = lock;
$("create").onclick = () => {
  createID = [...crypto.getRandomValues(new Uint8Array(12))]
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
  message("create-error", "");
  controls["create-form"].reset();
  changeGame();
  controls["create-dialog"].showModal();
};
$("close-create").onclick = () => controls["create-dialog"].close();
function changeGame() {
  const game = games.find((g) => g.id === controls["game"].value);
  controls["config"].value = game?.defaultConfig || "";
  if (game) settings(game, $("settings"), controls["config"]);
}
controls["game"].onchange = changeGame;
controls["create-form"].onsubmit = async (event) => {
  event.preventDefault();
  controls["launch"].disabled = true;
  message("create-error", "");
  const id = createID;
  try {
    const instance = await api<Instance>("/instances", {
      method: "POST",
      body: JSON.stringify({
        id,
        provider: controls["provider"].value,
        game: controls["game"].value,
        name: controls["name"].value,
        config: controls["config"].value,
        idleMinutes: Number(controls["idle"].value),
        maxMinutes: Number(controls["max"].value),
      }),
    });
    controls["create-dialog"].close();
    location.hash = "server/" + instance.id;
    await refresh(true);
  } catch (err) {
    message("create-error", errorMessage(err));
  } finally {
    controls["launch"].disabled = false;
  }
};
window.addEventListener("hashchange", () => route(true));
if (token) unlock().catch(() => lock());
else lock();
setInterval(() => refresh(), 5000);
