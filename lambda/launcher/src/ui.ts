const GAMES = ["xonotic", "qssm", "q2repro", "bzflag"];

export function renderUi(): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>insta-game</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { background: #111; color: #eee; font-family: monospace; padding: 2rem; }
    h1 { margin-bottom: 2rem; font-size: 1.4rem; }
    #auth { max-width: 400px; }
    #auth input { width: 100%; padding: .5rem; background: #222; color: #eee; border: 1px solid #444; margin-bottom: .5rem; }
    #auth button { padding: .5rem 1rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; }
    .games { display: flex; gap: 1rem; flex-wrap: wrap; }
    .game { background: #1a1a1a; border: 1px solid #333; padding: 1rem; min-width: 200px; }
    .game h2 { margin-bottom: .75rem; font-size: 1rem; }
    .status { margin-bottom: .75rem; font-size: .85rem; color: #aaa; }
    .status.online { color: #4f4; }
    .status.starting { color: #fa4; }
    .actions { display: flex; gap: .5rem; }
    .actions button { padding: .4rem .8rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; }
    .actions button:hover { background: #444; }
  </style>
</head>
<body>
  <h1>insta-game</h1>

  <div id="auth">
    <input type="password" id="passphrase" placeholder="passphrase" />
    <button onclick="authenticate()">unlock</button>
  </div>

  <div id="panel" style="display:none">
    <div class="games">
      ${GAMES.map(g => `
      <div class="game" id="game-${g}">
        <h2>${g}</h2>
        <div class="status" id="status-${g}">loading...</div>
        <div class="actions">
          <button onclick="action('${g}', 'start')">start</button>
          <button onclick="action('${g}', 'stop')">stop</button>
          <button onclick="refresh('${g}')">status</button>
        </div>
      </div>`).join("")}
    </div>
  </div>

  <script>
    const SESSION_KEY = "insta-game-passphrase";
    let passphrase = sessionStorage.getItem(SESSION_KEY) ?? "";

    function authenticate() {
      passphrase = document.getElementById("passphrase").value;
      sessionStorage.setItem(SESSION_KEY, passphrase);
      showPanel();
    }

    function showPanel() {
      if (!passphrase) return;
      document.getElementById("auth").style.display = "none";
      document.getElementById("panel").style.display = "";
      ${GAMES.map(g => `refresh("${g}");`).join(" ")}
    }

    function logout() {
      sessionStorage.removeItem(SESSION_KEY);
      passphrase = "";
      document.getElementById("auth").style.display = "";
      document.getElementById("panel").style.display = "none";
    }

    async function refresh(game) {
      const el = document.getElementById("status-" + game);
      try {
        const res = await fetch("/", {
          method: "POST",
          headers: { "X-Passphrase": passphrase, "Content-Type": "application/json" },
          body: JSON.stringify({ game, operation: "status" }),
        });
        if (res.status === 401) { logout(); return; }
        const data = await res.json();
        el.className = "status " + data.status;
        el.textContent = data.status + (data.publicIp ? " — " + data.publicIp : "") + (data.players ? " (" + data.players + " players)" : "");
      } catch {
        el.textContent = "error";
      }
    }

    async function action(game, op) {
      const el = document.getElementById("status-" + game);
      el.className = "status starting";
      el.textContent = op === "start" ? "starting..." : "stopping...";
      try {
        const res = await fetch("/", {
          method: "POST",
          headers: { "X-Passphrase": passphrase, "Content-Type": "application/json" },
          body: JSON.stringify({ game, operation: op }),
        });
        if (res.status === 401) { logout(); return; }
        const data = await res.json();
        el.className = "status " + data.status;
        el.textContent = data.status + (data.publicIp ? " — " + data.publicIp : "");
      } catch {
        el.textContent = "error";
      }
    }

    // Poll all games every 10s
    setInterval(() => { if (passphrase) { ${GAMES.map(g => `refresh("${g}")`).join("; ")}; } }, 10000);

    // Auto-show panel if passphrase already in sessionStorage
    if (passphrase) showPanel();
  </script>
</body>
</html>`;
}
