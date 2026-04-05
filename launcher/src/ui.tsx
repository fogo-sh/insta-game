/** @jsxImportSource hono/jsx */
import type { FC } from "hono/jsx";
import type { CachedGameState } from "./backend.js";

const SESSION_KEY = "insta-game-passphrase";

const css = `
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { background: #111; color: #eee; font-family: monospace; padding: 2rem; }
  .title-bar { display: flex; align-items: center; gap: 1.5rem; margin-bottom: 2rem; flex-wrap: wrap; }
  h1 { font-size: 1.4rem; }
  #auth-form { display: flex; align-items: center; gap: 0.5rem; }
  #auth-form input { padding: 0.35rem 0.5rem; background: #222; color: #eee; border: 1px solid #444; font-family: monospace; font-size: 0.85rem; width: 12rem; }
  #auth-form button { padding: 0.35rem 0.7rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; font-size: 0.85rem; }
  #auth-status { font-size: 0.8rem; color: #aaa; }

  .accordion { display: flex; flex-direction: column; gap: 0.5rem; }

  .row { border: 1px solid #333; background: #1a1a1a; }
  .row-header {
    display: flex; align-items: center; gap: 1rem;
    padding: 0.75rem 1rem; cursor: pointer; user-select: none;
    width: 100%;
  }
  .row-header:hover { background: #222; }
  .status-dot { font-size: 0.8rem; flex-shrink: 0; }
  .game-name { font-weight: bold; min-width: 8rem; }
  .row-meta { display: flex; gap: 1.5rem; flex: 1; color: #aaa; font-size: 0.85rem; flex-wrap: wrap; }
  .row-meta .online { color: #4f4; }
  .row-meta .starting { color: #fa4; }
  .row-meta .offline { color: #666; }
  .expand-btn {
    background: none; border: none; color: #aaa; cursor: pointer;
    font-family: monospace; font-size: 0.85rem; padding: 0; flex-shrink: 0;
  }

  .row-body { border-top: 1px solid #333; padding: 1rem; display: none; }
  .row-body.open { display: block; }

  .row-details { display: flex; gap: 2rem; align-items: flex-start; flex-wrap: wrap; margin-bottom: 1rem; }
  .connect code { background: #222; padding: 0.2rem 0.5rem; border: 1px solid #444; cursor: pointer; }
  .connect code:hover { background: #2a2a2a; }
  .client-link { font-size: 0.85rem; color: #aaa; }
  .client-link a { color: #88f; text-decoration: none; }
  .client-link a:hover { text-decoration: underline; }

  .admin-section { margin-top: 0.75rem; border-top: 1px solid #222; padding-top: 0.75rem; display: none; }
  .admin-section.unlocked { display: block; }
  .admin-controls { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: center; }
  .admin-controls button { padding: 0.4rem 0.8rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; }
  .admin-controls button:hover { background: #444; }

  .status-frag { font-size: 0.85rem; color: #aaa; margin-top: 0.5rem; min-height: 1.4em; }
  .status-frag .online { color: #4f4; }
  .status-frag .starting { color: #fa4; }
  .htmx-indicator { opacity: 0; transition: opacity 200ms ease-in; }
  .htmx-request .htmx-indicator { opacity: 1; }

  .log-section { margin-top: 0.75rem; border-top: 1px solid #222; padding-top: 0.75rem; display: none; }
  .log-section.open { display: block; }
  .log-panel { height: 300px; overflow-y: scroll; background: #0a0a0a; font-size: 0.75rem; padding: 0.75rem; border: 1px solid #222; }
  .log-line { white-space: pre-wrap; word-break: break-all; line-height: 1.5; }
`;

const initScript = `
(function() {
  var SESSION_KEY = ${JSON.stringify(SESSION_KEY)};

  function getPassphrase() {
    return sessionStorage.getItem(SESSION_KEY) || "";
  }

  // Toggle accordion open/close
  window.toggleRow = function(game) {
    var body = document.getElementById("row-body-" + game);
    var btn = document.getElementById("expand-btn-" + game);
    var open = body.classList.toggle("open");
    btn.textContent = open ? "[collapse ▲]" : "[expand ▼]";
  };

  // Copy connect address to clipboard
  window.copyConnect = function(text) {
    navigator.clipboard.writeText(text).catch(function() {});
  };

  // Unlock all admin sections — set hx-headers then show them
  function unlockAll(pp) {
    document.querySelectorAll(".admin-section").forEach(function(section) {
      section.setAttribute("hx-headers", JSON.stringify({"X-Passphrase": pp}));
      section.classList.add("unlocked");
      htmx.process(section);
    });
    document.getElementById("auth-form").style.display = "none";
    var status = document.getElementById("auth-status");
    status.style.display = "";
    status.textContent = "admin";
  }

  // Top-level authenticate
  window.authenticate = function() {
    var input = document.getElementById("passphrase-input");
    var btn = document.getElementById("passphrase-btn");
    var val = input.value;
    if (!val) return;
    btn.disabled = true;
    btn.textContent = "checking...";
    fetch("/", { headers: { "X-Passphrase": val, "HX-Request": "true" } })
      .then(function(res) {
        if (res.status === 401) {
          btn.disabled = false;
          btn.textContent = "unlock";
          input.style.borderColor = "#f44";
          return;
        }
        sessionStorage.setItem(SESSION_KEY, val);
        unlockAll(val);
      })
      .catch(function() {
        btn.disabled = false;
        btn.textContent = "unlock";
      });
  };

  // Toggle inline log panel open/closed
  window.toggleLogs = function(game) {
    var section = document.getElementById("log-section-" + game);
    var isOpen = section.classList.toggle("open");
    if (isOpen) {
      var pp = getPassphrase();
      var inner = document.getElementById("log-sse-" + game);
      if (!inner.getAttribute("sse-connect")) {
        inner.setAttribute("hx-ext", "sse");
        inner.setAttribute("sse-connect", "/logs?game=" + game + "&token=" + encodeURIComponent(pp));
        htmx.process(inner);
        var observer = new MutationObserver(function() { inner.scrollTop = inner.scrollHeight; });
        observer.observe(inner, { childList: true });
      }
    }
  };

  // Restore auth on page load
  (function() {
    var pp = getPassphrase();
    if (!pp) return;
    fetch("/", { headers: { "X-Passphrase": pp, "HX-Request": "true" } })
      .then(function(res) {
        if (res.status === 401) { sessionStorage.removeItem(SESSION_KEY); return; }
        unlockAll(pp);
      });
  })();
})();
`;

interface StatusDotProps { status: string }
const StatusDot: FC<StatusDotProps> = ({ status }) => {
  if (status === "online") return <span class="status-dot">🟢</span>;
  if (status === "starting") return <span class="status-dot">🟡</span>;
  return <span class="status-dot">⚫</span>;
};

interface RowProps {
  game: string;
  state: CachedGameState;
  connectAddress: string | null;
  clientDownloadUrl: string | null;
}

const AccordionRow: FC<RowProps> = ({ game, state, connectAddress, clientDownloadUrl }) => {
  const metaOnline = state.status === "online";
  const indicator = `#status-result-${game}`;
  return (
    <div class="row" id={`row-${game}`}>
      {/* Collapsed header — always visible, auto-refreshed by htmx every 30s */}
      <div
        id={`row-header-${game}`}
        class="row-header"
        hx-get={`/status?game=${game}`}
        hx-trigger="every 30s"
        hx-target={`#row-header-${game}`}
        hx-swap="outerHTML"
        onclick={`toggleRow('${game}')`}
      >
        <StatusDot status={state.status} />
        <span class="game-name">{game}</span>
        <span class="row-meta">
          {metaOnline && state.hostname ? <span>{state.hostname}</span> : null}
          {metaOnline && state.map ? <span>{state.map}</span> : null}
          {metaOnline ? <span>{state.players} player{state.players !== 1 ? "s" : ""}</span> : null}
          {!metaOnline ? <span class={state.status}>{state.status}</span> : null}
        </span>
        <button class="expand-btn" id={`expand-btn-${game}`}>[expand ▼]</button>
      </div>

      {/* Expanded body */}
      <div class="row-body" id={`row-body-${game}`}>
        <div class="row-details">
          {connectAddress ? (
            <div class="connect">
              connect: <code onclick={`copyConnect(${JSON.stringify(connectAddress)})`} title="click to copy">{connectAddress}</code>
            </div>
          ) : null}
          {clientDownloadUrl ? (
            <div class="client-link">
              <a href={clientDownloadUrl} target="_blank" rel="noopener">get client ↗</a>
            </div>
          ) : null}
        </div>

        {/* Admin controls — rendered in HTML, hidden until unlockAll() runs */}
        <div class="admin-section" id={`admin-section-${game}`}>
          <div class="admin-controls">
            <button
              hx-post={`/?game=${game}&operation=start`}
              hx-target={indicator}
              hx-indicator={indicator}
              hx-disabled-elt="this"
            >start</button>
            <button
              hx-post={`/?game=${game}&operation=stop`}
              hx-target={indicator}
              hx-indicator={indicator}
              hx-disabled-elt="this"
            >stop</button>
            <button type="button" onclick={`toggleLogs('${game}')`}>logs</button>
          </div>
          <div id={`status-result-${game}`} class="status-frag">
            <span class="htmx-indicator">working...</span>
          </div>
        </div>

        {/* Inline log panel */}
        <div class="log-section" id={`log-section-${game}`}>
          {/* log-sse-* gets hx-ext="sse" + sse-connect set dynamically by toggleLogs()
               sse-swap and hx-swap live on the same element so htmx.process() picks them up together */}
          <div id={`log-sse-${game}`} class="log-panel" sse-swap="log" hx-swap="beforeend" />
        </div>
      </div>
    </div>
  );
};

export interface GameUiConfig {
  connectAddress: string | null;
  clientDownloadUrl: string | null;
}

export function renderUi(
  games: Array<{ key: string; state: CachedGameState; ui: GameUiConfig }>
): string {
  const page = (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>insta-game</title>
        <style>{css}</style>
      </head>
      <body>
        <div class="title-bar">
          <h1>insta-game</h1>
          <form id="auth-form" onsubmit="authenticate(); return false;">
            <input
              type="text"
              id="passphrase-input"
              placeholder="passphrase"
              autocomplete="off"
              spellcheck={false}
              style="letter-spacing:0.15em;"
              oninput="this.style.borderColor=''"
            />
            <button id="passphrase-btn" type="submit">unlock</button>
          </form>
          <span id="auth-status" style="display:none" />
        </div>
        <div class="accordion">
          {games.map(({ key, state, ui }) => (
            <AccordionRow
              key={key}
              game={key}
              state={state}
              connectAddress={ui.connectAddress}
              clientDownloadUrl={ui.clientDownloadUrl}
            />
          ))}
        </div>
        <script src="https://unpkg.com/htmx.org@2/dist/htmx.min.js" />
        <script src="https://unpkg.com/htmx-ext-sse@2/sse.js" />
        <script dangerouslySetInnerHTML={{ __html: initScript }} />
      </body>
    </html>
  );
  return "<!DOCTYPE html>" + page.toString();
}

export function renderRowHeader(
  game: string,
  state: CachedGameState
): string {
  const metaOnline = state.status === "online";
  const dot = state.status === "online" ? "🟢" : state.status === "starting" ? "🟡" : "⚫";
  const frag = (
    <div
      id={`row-header-${game}`}
      class="row-header"
      hx-get={`/status?game=${game}`}
      hx-trigger="every 30s"
      hx-target={`#row-header-${game}`}
      hx-swap="outerHTML"
      onclick={`toggleRow('${game}')`}
    >
      <span class="status-dot">{dot}</span>
      <span class="game-name">{game}</span>
      <span class="row-meta">
        {metaOnline && state.hostname ? <span>{state.hostname}</span> : null}
        {metaOnline && state.map ? <span>{state.map}</span> : null}
        {metaOnline ? <span>{state.players} player{state.players !== 1 ? "s" : ""}</span> : null}
        {!metaOnline ? <span class={state.status}>{state.status}</span> : null}
      </span>
      <button class="expand-btn" id={`expand-btn-${game}`}>[expand ▼]</button>
    </div>
  );
  return frag.toString();
}
