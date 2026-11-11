import { serverState, stateLabel, serverActions } from "/status.js";
import { settings } from "./settings.js";
const $ = (id) => document.getElementById(id);
let token = sessionStorage.getItem("insta-game-token") || "";
let games = [],
  instances = [],
  logAbort,
  selectedID,
  createID;
function stopLogs() {
  logAbort?.abort();
  logAbort = null;
}
async function api(path, options = {}) {
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
function message(id, text) {
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
function connectAddress(i) {
  if (serverState(i) !== "running" || i.players == null) return "";
  return i.endpoints?.find((e) => e.port === i.game.gamePort)?.address || "";
}
function statusLabel(i) {
  return stateLabel(serverState(i));
}
function renderList() {
  const list = $("servers");
  list.replaceChildren();
  $("summary").textContent =
    `${instances.length} ${instances.length === 1 ? "server" : "servers"}`;
  if (!instances.length) {
    const empty = document.createElement("div");
    empty.className = "empty";
    empty.textContent = "No servers.";
    list.append(empty);
    return;
  }
  for (const i of [...instances].sort((a, b) =>
    b.createdAt.localeCompare(a.createdAt),
  )) {
    const row = $("server-template").content.cloneNode(true);
    const name = row.querySelector(".server-name");
    name.textContent = i.name;
    name.href = "#server/" + i.id;
    row.querySelector(".server-meta").textContent =
      i.game.displayName + " / " + i.provider;
    const state = row.querySelector(".phase");
    state.textContent = statusLabel(i);
    state.dataset.phase = serverState(i);
    row.querySelector("code").textContent =
      connectAddress(i) || "Address pending";
    row.querySelector(".players").textContent =
      (i.players === null ? "Players unknown" : `${i.players} players`) +
      (i.map ? " · " + i.map : "");
    list.append(row);
  }
}
function element(tag, text, className) {
  const e = document.createElement(tag);
  if (text) e.textContent = text;
  if (className) e.className = className;
  return e;
}
function runtimeNotice(i) {
  return i.stopReason || `idle timeout: ${i.idleMinutes ? i.idleMinutes + " min" : "off"}; runtime limit: ${i.maxMinutes ? i.maxMinutes + " min" : "off"}`;
}
function renderActions(i, actions) {
  actions.replaceChildren();
  for (const { label, action, disabled } of serverActions(i)) {
    const button = element(
      "button",
      label,
      action === "remove" ? "danger quiet" : "",
    );
    button.disabled = Boolean(disabled);
    button.dataset.action = action;
    button.onclick = async () => {
      if (action === "remove" && !confirm(`Remove ${i.name} and its saved data?`))
        return;
      try {
        button.disabled = true;
        await api(
          "/instances/" +
            i.id +
            (action === "remove" ? "" : "/" + action),
          { method: action === "remove" ? "DELETE" : "POST" },
        );
        await refresh(true);
      } catch (err) {
        message("detail-error", err.message);
        button.disabled = false;
      }
    };
    actions.append(button);
  }
  actions.dataset.state = JSON.stringify(serverActions(i));
}
function renderDetail(i) {
  const detail = $("detail");
  detail.replaceChildren();
  const back = element("a", "servers", "back");
  back.href = "#";
  detail.append(back);
  const heading = element("div", "", "detail-head");
  const title = element("div");
  title.append(
    element("p", i.game.displayName, "muted"),
    element("h2", i.name),
  );
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
      name === "status"
        ? "fact-status"
        : name === "address"
          ? "fact-address"
          : "fact-players";
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
      message(
        "detail-error",
        "Could not copy. Select the address to copy it manually.",
      );
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
      message("detail-error", err.message);
    }
  };
  panel.append(open, logs, commandForm);
  detail.append(panel);
  const config = element("details");
  config.append(
    element("summary", "config"),
    element("pre", i.config, "config-view"),
  );
  detail.append(config);
}
function updateDetail(i) {
  if (!$("fact-status")) return;
  $("fact-status").textContent = statusLabel(i);
  $("fact-players").textContent = `${i.players ?? "Unknown"} / ${i.map || "-"}`;
  $("fact-address").textContent = connectAddress(i) || "Pending";
  $("copy-address").disabled = !connectAddress(i);
  $("detail-error").textContent = i.error || "";
  const actions = $("detail").querySelector(".actions");
  const expected = serverActions(i);
  if (JSON.stringify(expected) !== actions.dataset.state) {
    renderActions(i, actions);
  }
  $("stop-reason").textContent = runtimeNotice(i);
}
async function streamLogs(id) {
  stopLogs();
  const stream = new AbortController();
  logAbort = stream;
  try {
    const response = await fetch("/api/instances/" + id + "/logs", {
      headers: { Authorization: "Bearer " + token },
      signal: stream.signal,
    });
    if (!response.ok) throw new Error("Could not open logs.");
    const reader = response.body.getReader(),
      decoder = new TextDecoder();
    let pending = "";
    $("log").textContent = "";
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      pending += decoder.decode(value, { stream: true });
      const lines = pending.split("\n");
      pending = lines.pop();
      for (const line of lines)
        if (line.startsWith("data: ")) {
          $("log").textContent += line.slice(6) + "\n";
        }
      $("log").textContent = $("log").textContent.slice(-100000);
      $("log").scrollTop = $("log").scrollHeight;
    }
  } catch (err) {
    if (err.name !== "AbortError" && logAbort === stream && $("log"))
      $("log").textContent = err.message;
  } finally {
    if (logAbort === stream) logAbort = null;
  }
}
function route(force = false) {
  const id = location.hash.startsWith("#server/")
    ? location.hash.slice(8)
    : null;
  if (!token) return;
  $("dashboard").hidden = Boolean(id);
  $("detail").hidden = !id;
  if (id) {
    const i = instances.find((i) => i.id === id);
    if (i) {
      if (id !== selectedID || force) {
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
    instances = await api("/instances");
    renderList();
    route(force);
    $("refresh-status").textContent =
      "Updated " + new Date().toLocaleTimeString();
    message("error", "");
    if (selectedID) {
      const current = instances.find((i) => i.id === selectedID);
      if (current) updateDetail(current);
    }
  } catch (err) {
    message("error", err.message);
    $("refresh-status").textContent = "Connection interrupted";
  }
}
async function unlock() {
  games = await api("/catalog");
 const targets = await api("/providers");
 $("provider").replaceChildren(...targets.providers.map(name => {
 const option = element("option", name); option.value = name; return option;
 }));
 $("provider").value = targets.default;
  $("game").replaceChildren(
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
$("login-form").onsubmit = async (event) => {
  event.preventDefault();
  token = $("token").value;
  try {
    await unlock();
    $("token").value = "";
  } catch (err) {
    message("login-error", err.message);
  }
};
$("signout").onclick = lock;
$("create").onclick = () => {
  createID = [...crypto.getRandomValues(new Uint8Array(12))]
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
  message("create-error", "");
  $("create-form").reset();
  changeGame();
  $("create-dialog").showModal();
};
$("close-create").onclick = () => $("create-dialog").close();
function changeGame() {
  const game = games.find((g) => g.id === $("game").value);
  $("config").value = game?.defaultConfig || "";
  if (game) settings(game, $("settings"), $("config"));
}
$("game").onchange = changeGame;
$("create-form").onsubmit = async (event) => {
  event.preventDefault();
  $("launch").disabled = true;
  message("create-error", "");
  const id = createID;
  try {
    const instance = await api("/instances", {
      method: "POST",
      body: JSON.stringify({
        id,
        provider: $("provider").value,
 game: $("game").value,
        name: $("name").value,
        config: $("config").value,
        idleMinutes: Number($("idle").value),
        maxMinutes: Number($("max").value),
      }),
    });
    $("create-dialog").close();
    location.hash = "server/" + instance.id;
    await refresh(true);
  } catch (err) {
    message("create-error", err.message);
  } finally {
    $("launch").disabled = false;
  }
};
window.addEventListener("hashchange", () => route(true));
if (token) unlock().catch(() => lock());
else lock();
setInterval(() => refresh(), 5000);
