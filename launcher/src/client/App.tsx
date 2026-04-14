import { h } from "preact";
import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { fetchStatus, validatePassphrase, type StatusResponse } from "./api.js";
import { GameRow } from "./GameRow.js";

const SESSION_KEY = "insta-game-passphrase";
const POLL_INTERVAL_MS = 10_000;
const PAUSE_AFTER_ACTION_MS = 15_000;
const BACKOFF_INTERVAL_MS = 30_000;

export function App() {
  const [passphrase, setPassphrase] = useState<string | null>(() =>
    sessionStorage.getItem(SESSION_KEY)
  );
  const [passphraseInput, setPassphraseInput] = useState("");
  const [authError, setAuthError] = useState(false);
  const [authPending, setAuthPending] = useState(false);
  const [games, setGames] = useState<StatusResponse | null>(null);
  const pauseUntilRef = useRef(0);
  const nextIntervalRef = useRef(POLL_INTERVAL_MS);

  // Validate stored passphrase on mount
  useEffect(() => {
    const stored = sessionStorage.getItem(SESSION_KEY);
    if (!stored) return;
    validatePassphrase(stored).then(ok => {
      if (!ok) {
        sessionStorage.removeItem(SESSION_KEY);
        setPassphrase(null);
      }
    });
  }, []);

  // Status polling
  useEffect(() => {
    let cancelled = false;

    async function poll() {
      if (cancelled) return;
      if (Date.now() < pauseUntilRef.current) {
        setTimeout(poll, Math.max(1000, pauseUntilRef.current - Date.now()));
        return;
      }
      try {
        const data = await fetchStatus();
        if (!cancelled) {
          setGames(data);
          nextIntervalRef.current = POLL_INTERVAL_MS;
        }
      } catch (err) {
        const status = (err as { status?: number }).status;
        if (status === 429) nextIntervalRef.current = BACKOFF_INTERVAL_MS;
      }
      if (!cancelled) setTimeout(poll, nextIntervalRef.current);
    }

    void poll();
    return () => { cancelled = true; };
  }, []);

  const handleAuth = useCallback(async (e: Event) => {
    e.preventDefault();
    if (!passphraseInput) return;
    setAuthPending(true);
    setAuthError(false);
    const ok = await validatePassphrase(passphraseInput);
    setAuthPending(false);
    if (!ok) {
      setAuthError(true);
      return;
    }
    sessionStorage.setItem(SESSION_KEY, passphraseInput);
    setPassphrase(passphraseInput);
    setPassphraseInput("");
  }, [passphraseInput]);

  const pausePolling = useCallback(() => {
    pauseUntilRef.current = Date.now() + PAUSE_AFTER_ACTION_MS;
  }, []);

  const sortedGames = games
    ? Object.entries(games).sort(([, a], [, b]) =>
        (a.displayName || "").localeCompare(b.displayName || "")
      )
    : null;

  return (
    <div>
      <div class="title-bar">
        <h1>insta-game</h1>
        {passphrase === null ? (
          <form id="auth-form" onSubmit={handleAuth}>
            <input
              type="text"
              value={passphraseInput}
              onInput={e => { setPassphraseInput((e.target as HTMLInputElement).value); setAuthError(false); }}
              placeholder="passphrase"
              autocomplete="off"
              spellcheck={false}
              style={authError ? "border-color: #f44; letter-spacing: 0.15em;" : "letter-spacing: 0.15em;"}
            />
            <button type="submit" disabled={authPending}>
              {authPending ? "checking..." : "unlock"}
            </button>
          </form>
        ) : (
          <span id="auth-status">admin</span>
        )}
      </div>

      <div class="accordion">
        {sortedGames === null ? (
          <div style="color: #666; font-size: 0.85rem;">loading...</div>
        ) : sortedGames.length === 0 ? (
          <div style="color: #666; font-size: 0.85rem;">no games configured</div>
        ) : (
          sortedGames.map(([id, game]) => (
            <GameRow
              key={id}
              id={id}
              game={game}
              passphrase={passphrase}
              onAction={pausePolling}
            />
          ))
        )}
      </div>
    </div>
  );
}
