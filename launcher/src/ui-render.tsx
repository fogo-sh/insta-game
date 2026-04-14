/** @jsxImportSource hono/jsx */
import type { FC } from "hono/jsx";
import type { CachedGameState } from "./backend.js";
import { initScript } from "./ui-client.js";
import {
  logPanelId,
  logSectionId,
  renderRowHeaderContent,
  rowBodyId,
  rowHeaderId,
} from "./ui-shared.js";
import { css } from "./ui-styles.js";

interface RowProps {
  game: string;
  displayName: string | null;
  state: CachedGameState;
  connectAddress: string | null;
  clientDownloadUrl: string | null;
  startBlocked: boolean;
  logsEnabled: boolean;
  logMode: "sse" | "poll";
  logUrl: string;
}

const AccordionRow: FC<RowProps> = ({ game, displayName, state, connectAddress, clientDownloadUrl, startBlocked, logsEnabled, logMode, logUrl }) => {
  const label = displayName ?? game;
  const indicator = `#status-result-${game}`;
  return (
    <div class="row" id={`row-${game}`}>
      <div
        id={rowHeaderId(game)}
        class="row-header"
        data-label={label}
        onclick={state.status !== "offline" ? `toggleRow('${game}')` : undefined}
        style={state.status === "offline" ? "cursor: default" : undefined}
        dangerouslySetInnerHTML={{ __html: renderRowHeaderContent(label, game, state) }}
      />

      <div class="row-body" id={rowBodyId(game)}>
        <div class="row-details">
          {connectAddress ? (
            <div class="connect">
              connect: <code id={`connect-${game}`}>{connectAddress}</code>
              <button type="button" class="copy-btn" onclick={`copyConnect(${JSON.stringify(connectAddress)}, this)`} title="copy to clipboard">copy</button>
            </div>
          ) : null}
          {clientDownloadUrl ? (
            <div class="client-link">
              <a href={clientDownloadUrl} target="_blank" rel="noopener">get client ↗</a>
            </div>
          ) : null}
        </div>

        <div class="admin-section" id={`admin-section-${game}`}>
          <div class="admin-controls">
            <button
              hx-post={`/?game=${game}&operation=start`}
              data-admin-action="start"
              hx-target={indicator}
              hx-indicator={indicator}
              hx-disabled-elt="this"
              disabled={startBlocked || undefined}
              title={startBlocked ? "a conflicting game is already running on the same port" : undefined}
            >start</button>
            <button
              hx-post={`/?game=${game}&operation=stop`}
              data-admin-action="stop"
              hx-target={indicator}
              hx-indicator={indicator}
              hx-disabled-elt="this"
            >stop</button>
            {logsEnabled ? <button type="button" onclick={`toggleLogs('${game}')`}>logs</button> : null}
          </div>
          <div id={`status-result-${game}`} class="status-frag">
            <span class="htmx-indicator">working...</span>
          </div>
        </div>

        <div class="log-section" id={logSectionId(game)}>
          <div
            id={logPanelId(game)}
            class="log-panel"
            data-log-mode={logMode}
            data-log-open="0"
            data-log-url={logUrl}
            data-log-cursor=""
            sse-swap="log"
            hx-swap="beforeend"
          />
        </div>
      </div>
    </div>
  );
};

export interface GameUiConfig {
  displayName: string | null;
  connectAddress: string | null;
  clientDownloadUrl: string | null;
  startBlocked: boolean;
  logsEnabled: boolean;
  logMode: "sse" | "poll";
  logUrl: string;
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
              displayName={ui.displayName}
              state={state}
              connectAddress={ui.connectAddress}
              clientDownloadUrl={ui.clientDownloadUrl}
              startBlocked={ui.startBlocked}
              logsEnabled={ui.logsEnabled}
              logMode={ui.logMode}
              logUrl={ui.logUrl}
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
