import { SESSION_KEY } from "./ui-shared.js";

export const initScript = `
(function() {
  var SESSION_KEY = ${JSON.stringify(SESSION_KEY)};
  var STATUS_POLL_INTERVAL_MS = 10000;
  var STATUS_RETRY_INTERVAL_MS = 30000;
  var LOG_POLL_INTERVAL_MS = 5000;
  var LOG_RETRY_INTERVAL_MS = 15000;
  var POLL_PAUSE_AFTER_ADMIN_MS = 15000;
  var suspendPollingUntil = 0;

  function getPassphrase() {
    return sessionStorage.getItem(SESSION_KEY) || "";
  }

  function statusDot(status) {
    if (status === "online") return "🟢";
    if (status === "starting") return "🟡";
    return "⚫";
  }

  function escapeHtml(text) {
    return String(text)
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;");
  }

  function renderRowHeader(label, game, state) {
    var meta = [];
    if (state.status === "online" && state.hostname) meta.push("<span>" + escapeHtml(state.hostname) + "</span>");
    if (state.status === "online" && state.map) meta.push("<span>" + escapeHtml(state.map) + "</span>");
    if (state.status === "online") meta.push("<span>" + state.players + " player" + (state.players !== 1 ? "s" : "") + "</span>");
    if (state.status !== "online") meta.push("<span class=\\"" + state.status + "\\">" + state.status + "</span>");
    return ""
      + "<span class=\\"status-dot\\">" + statusDot(state.status) + "</span>"
      + "<span class=\\"game-name\\">" + escapeHtml(label) + "</span>"
      + "<span class=\\"row-meta\\">" + meta.join("") + "</span>"
      + "<button class=\\"expand-btn\\" id=\\"expand-btn-" + game + "\\">[expand ▼]</button>";
  }

  function syncExpandButton(game) {
    var body = document.getElementById("row-body-" + game);
    var btn = document.getElementById("expand-btn-" + game);
    if (!body || !btn) return;
    btn.textContent = body.classList.contains("open") ? "[collapse ▲]" : "[expand ▼]";
  }

  function refreshStatuses() {
    if (Date.now() < suspendPollingUntil) {
      window.setTimeout(refreshStatuses, Math.max(1000, suspendPollingUntil - Date.now()));
      return;
    }
    var retryDelay = STATUS_POLL_INTERVAL_MS;
    fetch("/status")
      .then(function(res) {
        if (!res.ok) {
          var error = new Error("HTTP " + res.status);
          error.status = res.status;
          throw error;
        }
        return res.json();
      })
      .then(function(payload) {
        Object.entries(payload).forEach(function(entry) {
          var game = entry[0];
          var state = entry[1];
          var header = document.getElementById("row-header-" + game);
          if (!header) return;
          var label = header.getAttribute("data-label") || game;
          header.innerHTML = renderRowHeader(label, game, state);
          syncExpandButton(game);
        });
      })
      .catch(function(error) {
        if (error.status === 429) retryDelay = STATUS_RETRY_INTERVAL_MS;
      })
      .finally(function() {
        var delay = retryDelay;
        if (Date.now() < suspendPollingUntil) {
          delay = Math.max(1000, suspendPollingUntil - Date.now());
        }
        window.setTimeout(refreshStatuses, delay);
      });
  }

  function appendLogLines(inner, lines) {
    lines.forEach(function(line) {
      var div = document.createElement("div");
      div.className = "log-line";
      div.textContent = line;
      inner.appendChild(div);
    });
    inner.scrollTop = inner.scrollHeight;
  }

  function pollLogs(game, inner) {
    if (inner.getAttribute("data-log-mode") !== "poll") return;
    if (inner.getAttribute("data-log-open") !== "1") return;
    if (Date.now() < suspendPollingUntil) {
      window.setTimeout(function() { pollLogs(game, inner); }, Math.max(1000, suspendPollingUntil - Date.now()));
      return;
    }
    var pp = getPassphrase();
    var cursor = inner.getAttribute("data-log-cursor") || "";
    var url = inner.getAttribute("data-log-url");
    var retryDelay = LOG_POLL_INTERVAL_MS;
    if (!url) return;
    var sep = url.indexOf("?") >= 0 ? "&" : "?";
    fetch(url + sep + "token=" + encodeURIComponent(pp) + (cursor ? "&cursor=" + encodeURIComponent(cursor) : ""))
      .then(function(res) {
        if (!res.ok) {
          var error = new Error("HTTP " + res.status);
          error.status = res.status;
          throw error;
        }
        return res.json();
      })
      .then(function(payload) {
        if (Array.isArray(payload.lines) && payload.lines.length > 0) {
          appendLogLines(inner, payload.lines);
        }
        if (payload.cursor) inner.setAttribute("data-log-cursor", payload.cursor);
      })
      .catch(function(error) {
        if (error.status === 429) retryDelay = LOG_RETRY_INTERVAL_MS;
        if (error.status !== 429) {
          appendLogLines(inner, ["[log poll error: " + error.message + "]"]);
        }
      })
      .finally(function() {
        if (inner.getAttribute("data-log-open") === "1") {
          var delay = retryDelay;
          if (Date.now() < suspendPollingUntil) {
            delay = Math.max(1000, suspendPollingUntil - Date.now());
          }
          window.setTimeout(function() { pollLogs(game, inner); }, delay);
        }
      });
  }

  window.toggleRow = function(game) {
    var body = document.getElementById("row-body-" + game);
    var btn = document.getElementById("expand-btn-" + game);
    if (!body || !btn) return;
    var open = body.classList.toggle("open");
    btn.textContent = open ? "[collapse ▲]" : "[expand ▼]";
  };

  window.copyConnect = function(text) {
    navigator.clipboard.writeText(text).catch(function() {});
  };

  function unlockAll(pp) {
    document.querySelectorAll(".admin-section").forEach(function(section) {
      section.setAttribute("hx-headers", JSON.stringify({"X-Passphrase": pp}));
      section.classList.add("unlocked");
      htmx.process(section);
    });
    var authForm = document.getElementById("auth-form");
    var status = document.getElementById("auth-status");
    if (!authForm || !status) return;
    authForm.style.display = "none";
    status.style.display = "";
    status.textContent = "admin";
  }

  document.addEventListener("click", function(event) {
    var target = event.target;
    if (!(target instanceof Element)) return;
    if (!target.closest("[data-admin-action]")) return;
    suspendPollingUntil = Date.now() + POLL_PAUSE_AFTER_ADMIN_MS;
  });

  window.authenticate = function() {
    var input = document.getElementById("passphrase-input");
    var btn = document.getElementById("passphrase-btn");
    if (!(input instanceof HTMLInputElement) || !(btn instanceof HTMLButtonElement)) return;
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

  window.toggleLogs = function(game) {
    var section = document.getElementById("log-section-" + game);
    var inner = document.getElementById("log-sse-" + game);
    if (!section || !inner) return;
    var isOpen = section.classList.toggle("open");
    inner.setAttribute("data-log-open", isOpen ? "1" : "0");
    if (isOpen) {
      if (inner.getAttribute("data-log-mode") === "poll") {
        if (!inner.getAttribute("data-log-started")) {
          inner.setAttribute("data-log-started", "1");
          appendLogLines(inner, ["[connecting to " + game + " logs]"]);
          pollLogs(game, inner);
        }
      } else if (!inner.getAttribute("sse-connect")) {
        var pp = getPassphrase();
        var baseUrl = inner.getAttribute("data-log-url");
        var separator = baseUrl && baseUrl.indexOf("?") >= 0 ? "&" : "?";
        inner.setAttribute("hx-ext", "sse");
        inner.setAttribute("sse-connect", (baseUrl || "/logs?game=" + game) + separator + "token=" + encodeURIComponent(pp));
        htmx.process(inner);
        var observer = new MutationObserver(function() { inner.scrollTop = inner.scrollHeight; });
        observer.observe(inner, { childList: true });
      }
    }
  };

  (function() {
    refreshStatuses();
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
