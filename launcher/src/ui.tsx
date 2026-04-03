/** @jsxImportSource hono/jsx */
import type { FC } from "hono/jsx";

const SESSION_KEY = "insta-game-passphrase";

const css = `
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { background: #111; color: #eee; font-family: monospace; padding: 2rem; }
  h1 { margin-bottom: 2rem; font-size: 1.4rem; }
  #auth { max-width: 400px; }
  #auth input { width: 100%; padding: .5rem; background: #222; color: #eee; border: 1px solid #444; margin-bottom: .5rem; font-family: monospace; }
  #auth button { padding: .5rem 1rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; }
  .games { display: flex; gap: 1rem; flex-wrap: wrap; }
  .game { background: #1a1a1a; border: 1px solid #333; padding: 1rem; min-width: 200px; }
  .game h2 { margin-bottom: .75rem; font-size: 1rem; }
  .status { margin-bottom: .75rem; font-size: .85rem; color: #aaa; }
  .status.online { color: #4f4; }
  .status.starting { color: #fa4; }
  .actions { display: flex; gap: .5rem; flex-wrap: wrap; }
  .actions button { padding: .4rem .8rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; }
  .actions button:hover { background: #444; }
  dialog { background: #111; color: #eee; border: 1px solid #444; padding: 0; width: calc(100vw - 4rem); max-width: 960px; height: calc(100vh - 4rem); display: flex; flex-direction: column; }
  dialog::backdrop { background: rgba(0,0,0,0.7); }
  .dialog-header { display: flex; align-items: center; justify-content: space-between; padding: .75rem 1rem; border-bottom: 1px solid #333; font-size: .85rem; }
  .dialog-header span { color: #aaa; }
  .dialog-close { background: none; border: none; color: #aaa; cursor: pointer; font-size: 1.2rem; font-family: monospace; padding: 0 .25rem; }
  .dialog-close:hover { color: #eee; }
  .log-panel { flex: 1; overflow-y: scroll; background: #0a0a0a; font-size: 0.75rem; padding: 0.75rem; }
  .log-line { white-space: pre-wrap; word-break: break-all; line-height: 1.5; }
`;

// Small inline script — runs once on page load
const initScript = `
(function() {
  var SESSION_KEY = ${JSON.stringify(SESSION_KEY)};
  var passphrase = sessionStorage.getItem(SESSION_KEY) || "";

  function showPanel(pp) {
    var auth = document.getElementById("auth");
    var panel = document.getElementById("panel");
    auth.style.display = "none";
    panel.setAttribute("hx-headers", JSON.stringify({"X-Passphrase": pp}));
    panel.style.display = "";
    htmx.process(panel);
  }

  window.authenticate = function() {
    var val = document.getElementById("passphrase").value;
    if (!val) return;
    sessionStorage.setItem(SESSION_KEY, val);
    passphrase = val;
    showPanel(val);
  };

  window.toggleLogs = function(game) {
    var dialog = document.getElementById("log-dialog-" + game);
    if (dialog.open) {
      dialog.close();
      return;
    }
    var pp = sessionStorage.getItem(SESSION_KEY) || "";
    var inner = document.getElementById("log-sse-" + game);
    // Only initialise the SSE connection once
    if (!inner.getAttribute("sse-connect")) {
      inner.setAttribute("sse-connect", "/logs?game=" + game + "&token=" + encodeURIComponent(pp));
      htmx.process(inner);
      // Auto-scroll on new content
      var lines = document.getElementById("log-lines-" + game);
      var observer = new MutationObserver(function() {
        lines.scrollTop = lines.scrollHeight;
      });
      observer.observe(lines, { childList: true });
    }
    dialog.showModal();
  };

  window.logout = function() {
    sessionStorage.removeItem(SESSION_KEY);
    location.reload();
  };

  // Auto-show panel if passphrase already stored
  if (passphrase) showPanel(passphrase);
})();
`;

interface GameCardProps {
  game: string;
}

const GameCard: FC<GameCardProps> = ({ game }) => (
  <div class="game" id={`game-${game}`}>
    <h2>{game}</h2>
    {/* Status fragment — htmx will swap this via GET /?game=...&operation=status */}
    <div class="status" id={`status-${game}`}
      hx-get={`/?game=${game}&operation=status`}
      hx-trigger="load, every 10s"
      hx-target={`#status-${game}`}
    >
      loading...
    </div>
    <div class="actions">
      <button
        hx-post={`/?game=${game}&operation=start`}
        hx-target={`#status-${game}`}
      >start</button>
      <button
        hx-post={`/?game=${game}&operation=stop`}
        hx-target={`#status-${game}`}
      >stop</button>
      <button onclick={`toggleLogs('${game}')`}>logs</button>
    </div>
  </div>
);

export function renderUi(games: string[]): string {
  const page = (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>insta-game</title>
        <style>{css}</style>
      </head>
      <body>
        <h1>insta-game</h1>

        <div id="auth">
          {/* type="text" + autocomplete="off" avoids 1Password false-positive */}
          <input
            type="text"
            id="passphrase"
            placeholder="passphrase"
            autocomplete="off"
            spellcheck={false}
            style="letter-spacing: 0.15em;"
          />
          <button onclick="authenticate()">unlock</button>
        </div>

        <div id="panel" style="display:none" hx-headers="{}">
          <div class="games">
            {games.map(g => <GameCard key={g} game={g} />)}
          </div>
          <br />
          <button onclick="logout()" style="margin-top:1rem;padding:.3rem .7rem;background:#222;color:#888;border:1px solid #444;cursor:pointer;font-family:monospace;font-size:0.8rem;">logout</button>
        </div>

        {/* Log dialogs — one per game, opened by toggleLogs() */}
        {games.map(g => (
          <dialog id={`log-dialog-${g}`} key={g}>
            <div class="dialog-header">
              <span>{g} — logs</span>
              <button class="dialog-close" onclick={`document.getElementById('log-dialog-${g}').close()`}>✕</button>
            </div>
            <div id={`log-sse-${g}`} hx-ext="sse">
              <div
                id={`log-lines-${g}`}
                class="log-panel"
                sse-swap="log"
                hx-swap="beforeend"
              />
            </div>
          </dialog>
        ))}

        <script src="https://unpkg.com/htmx.org@2/dist/htmx.min.js" />
        <script src="https://unpkg.com/htmx-ext-sse@2/sse.js" />
        <script dangerouslySetInnerHTML={{ __html: initScript }} />
      </body>
    </html>
  );

  return "<!DOCTYPE html>" + page.toString();
}
