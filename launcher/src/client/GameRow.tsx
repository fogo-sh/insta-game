import { h } from "preact";
import { useState, useCallback } from "preact/hooks";
import { postAction, type GameEntry } from "./api.js";
import { LogPanel } from "./LogPanel.js";

interface GameRowProps {
  id: string;
  game: GameEntry;
  passphrase: string | null;
  onAction: () => void;
}

function statusDot(status: string): string {
  if (status === "online") return "🟢";
  if (status === "starting") return "🟡";
  return "⚫";
}

export function GameRow({ id, game, passphrase, onAction }: GameRowProps) {
  const [open, setOpen] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [actionResult, setActionResult] = useState<{ message: string; ok: boolean } | null>(null);
  const [acting, setActing] = useState(false);

  const expandable = game.status !== "offline" || passphrase !== null;

  const toggle = useCallback(() => {
    if (!expandable) return;
    setOpen(o => !o);
  }, [expandable]);

  const handleAction = useCallback(async (operation: "start" | "stop") => {
    if (!passphrase) return;
    setActing(true);
    setActionResult(null);
    onAction();
    try {
      const result = await postAction(id, operation, passphrase);
      setActionResult({ message: `${operation} → ${result.status}`, ok: result.status !== "offline" });
    } catch (err) {
      setActionResult({ message: `${operation} failed: ${err instanceof Error ? err.message : String(err)}`, ok: false });
    } finally {
      setActing(false);
    }
  }, [id, passphrase, onAction]);

  const copyConnect = useCallback((address: string, e: MouseEvent) => {
    void navigator.clipboard.writeText(address);
    const btn = e.currentTarget as HTMLButtonElement;
    const orig = btn.textContent;
    btn.textContent = "✓";
    btn.classList.add("copied");
    setTimeout(() => {
      btn.textContent = orig;
      btn.classList.remove("copied");
    }, 1500);
  }, []);

  return (
    <div class="row">
      <div
        class={`row-header${expandable ? "" : " not-expandable"}`}
        onClick={toggle}
      >
        <span class="status-dot">{statusDot(game.status)}</span>
        <span class="game-name">{game.displayName || id}</span>
        <span class="row-meta">
          {game.status === "online" && game.hostname ? <span>{game.hostname}</span> : null}
          {game.status === "online" && game.map ? <span>{game.map}</span> : null}
          {game.status === "online" ? (
            <span>{game.players} player{game.players !== 1 ? "s" : ""}</span>
          ) : (
            <span class={game.status}>{game.status}</span>
          )}
        </span>
        {expandable ? (
          <button class="expand-btn" onClick={e => { e.stopPropagation(); toggle(); }}>
            {open ? "[collapse ▲]" : "[expand ▼]"}
          </button>
        ) : null}
      </div>

      {open ? (
        <div class="row-body open">
          <div class="row-details">
            {game.connectAddress ? (
              <div class="connect">
                connect: <code>{game.connectAddress}</code>
                <button
                  class="copy-btn"
                  title="copy to clipboard"
                  onClick={e => copyConnect(game.connectAddress!, e)}
                >copy</button>
              </div>
            ) : null}
            {game.clientDownloadUrl ? (
              <div class="client-link">
                <a href={game.clientDownloadUrl} target="_blank" rel="noopener">get client ↗</a>
              </div>
            ) : null}
          </div>

          {passphrase ? (
            <div class="admin-section">
              <div class="admin-controls">
                <button
                  onClick={() => void handleAction("start")}
                  disabled={acting || game.startBlocked}
                  title={game.startBlocked ? "a conflicting game is already running on the same port" : undefined}
                >start</button>
                <button
                  onClick={() => void handleAction("stop")}
                  disabled={acting}
                >stop</button>
                <button
                  type="button"
                  onClick={() => setLogsOpen(l => !l)}
                >{logsOpen ? "hide logs" : "logs"}</button>
              </div>
              {actionResult ? (
                <div class={`action-result ${actionResult.ok ? "ok" : "err"}`}>
                  {actionResult.message}
                </div>
              ) : null}
            </div>
          ) : null}

          {logsOpen && passphrase ? (
            <div class="log-section">
              <LogPanel game={id} passphrase={passphrase} />
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
