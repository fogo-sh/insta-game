import { h } from "preact";
import { useState, useCallback, useEffect, useRef } from "preact/hooks";
import { postAction, fetchConfigEditor, type GameEntry, type ConfigEditorResponse } from "./api.js";
import { LogPanel } from "./LogPanel.js";
import {
  applyManagedConfig,
  buildInitialConfigEditorState,
  findControlSelection,
  type ConfigEditorControl,
  type ConfigEditorDefinition,
  type ConfigEditorState,
} from "../config-editor.js";

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

function updateConfigFromControl(
  configText: string,
  configEditor: ConfigEditorDefinition | null,
  formState: ConfigEditorState | null,
  control: ConfigEditorControl,
  nextValue: boolean | number | string | string[]
): { configText: string; formState: ConfigEditorState | null } {
  if (!configEditor || !formState) {
    return { configText, formState };
  }
  const nextFormState = {
    ...formState,
    [control.id]: nextValue,
  };
  return {
    formState: nextFormState,
    configText: applyManagedConfig(configText, configEditor, nextFormState),
  };
}

function renderControl(
  control: ConfigEditorControl,
  formState: ConfigEditorState,
  configText: string,
  configEditor: ConfigEditorDefinition | null,
  setFormState: (value: ConfigEditorState | null) => void,
  setConfigText: (value: string) => void,
  focusConfigSelection: (nextText: string, nextState: ConfigEditorState | null, controlId: string) => void
) {
  const value = formState[control.id];

  const commitValue = (
    nextValue: boolean | number | string | string[],
    spotlight = true
  ) => {
    const next = updateConfigFromControl(configText, configEditor, formState, control, nextValue);
    setFormState(next.formState);
    setConfigText(next.configText);
    if (spotlight) {
      focusConfigSelection(next.configText, next.formState, control.id);
    }
  };

  switch (control.type) {
    case "checkbox":
      return (
        <label class="config-check" key={control.id}>
          <input
            type="checkbox"
            checked={Boolean(value)}
            onInput={e => {
              commitValue((e.target as HTMLInputElement).checked);
            }}
          />
          <span>{control.label}</span>
        </label>
      );
    case "radio":
      return (
        <fieldset class="config-fieldset" key={control.id}>
          <legend>{control.label}</legend>
          <div class="config-radio-group">
            {control.options.map(option => (
              <label class="config-check" key={option.value}>
                <input
                  type="radio"
                  name={control.id}
                  checked={value === option.value}
                  onInput={() => {
                    commitValue(option.value);
                  }}
                />
                <span>{option.label}</span>
              </label>
            ))}
          </div>
        </fieldset>
      );
    case "number":
      return (
        <label class="config-field" key={control.id}>
          <span>{control.label}</span>
          <input
            type="number"
            value={String(value)}
            min={control.min}
            max={control.max}
            step={control.step}
            onInput={e => {
              commitValue(Number((e.target as HTMLInputElement).value), false);
            }}
            onBlur={() => {
              focusConfigSelection(configText, formState, control.id);
            }}
          />
        </label>
      );
    case "text":
      return (
        <label class="config-field" key={control.id}>
          <span>{control.label}</span>
          <input
            type="text"
            value={String(value)}
            placeholder={control.placeholder}
            onInput={e => {
              commitValue((e.target as HTMLInputElement).value, false);
            }}
            onBlur={() => {
              focusConfigSelection(configText, formState, control.id);
            }}
          />
        </label>
      );
    case "multiselect":
      return (
        <fieldset class="config-fieldset" key={control.id}>
          <legend>{control.label}</legend>
          <div class="config-checkbox-grid">
            {control.options.map(option => {
              const selected = Array.isArray(value) && value.includes(option.value);
              return (
                <label class="config-check" key={option.value}>
                  <input
                    type="checkbox"
                    checked={selected}
                    onInput={e => {
                      const checked = (e.target as HTMLInputElement).checked;
                      const current = Array.isArray(value) ? value.filter(item => item !== option.value) : [];
                      commitValue(checked ? [...current, option.value] : current);
                    }}
                  />
                  <span>{option.label}</span>
                </label>
              );
            })}
          </div>
        </fieldset>
      );
  }
}

export function GameRow({ id, game, passphrase, onAction }: GameRowProps) {
  const [open, setOpen] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [actionResult, setActionResult] = useState<{ message: string; ok: boolean } | null>(null);
  const [acting, setActing] = useState(false);
  const [editorLoading, setEditorLoading] = useState(false);
  const [editorError, setEditorError] = useState<string | null>(null);
  const [configText, setConfigText] = useState<string | null>(null);
  const [configEditor, setConfigEditor] = useState<ConfigEditorDefinition | null>(null);
  const [formState, setFormState] = useState<ConfigEditorState | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  const expandable = game.status !== "offline" || passphrase !== null;

  const focusConfigSelection = useCallback((
    nextText: string,
    nextState: ConfigEditorState | null,
    controlId: string
  ) => {
    if (!configEditor || !nextState) return;
    requestAnimationFrame(() => {
      const textarea = textareaRef.current;
      if (!textarea) return;
      const selection = findControlSelection(nextText, configEditor, nextState, controlId);
      textarea.focus();
      if (selection) {
        textarea.setSelectionRange(selection.start, selection.end);
        const lineHeight = parseFloat(window.getComputedStyle(textarea).lineHeight || "20");
        const lineNumber = nextText.slice(0, selection.start).split("\n").length - 1;
        textarea.scrollTop = Math.max(0, lineNumber * lineHeight - textarea.clientHeight / 3);
      }
      textarea.classList.remove("config-flash");
      void textarea.offsetWidth;
      textarea.classList.add("config-flash");
    });
  }, [configEditor]);

  useEffect(() => {
    if (!open || !passphrase || configText !== null) return;
    let cancelled = false;
    setEditorLoading(true);
    setEditorError(null);
    fetchConfigEditor(id, passphrase)
      .then((response: ConfigEditorResponse) => {
        if (cancelled) return;
        setConfigText(response.configText);
        setConfigEditor(response.configEditor);
        setFormState(response.configEditor ? buildInitialConfigEditorState(response.configEditor) : null);
      })
      .catch(error => {
        if (cancelled) return;
        setEditorError(error instanceof Error ? error.message : String(error));
      })
      .finally(() => {
        if (!cancelled) setEditorLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [configText, id, open, passphrase]);

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
      const result = await postAction(id, operation, passphrase, operation === "start" ? (configText ?? undefined) : undefined);
      setActionResult({ message: `${operation} → ${result.status}`, ok: result.status !== "offline" });
    } catch (err) {
      setActionResult({ message: `${operation} failed: ${err instanceof Error ? err.message : String(err)}`, ok: false });
    } finally {
      setActing(false);
    }
  }, [configText, id, passphrase, onAction]);

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

              {configText !== null ? (
                <div class="config-editor">
                  {configEditor && formState ? (
                    <div class="config-controls">
                      {configEditor.controls.map(control => (
                        <div class="config-control" key={control.id}>
                          {renderControl(
                            control,
                            formState,
                            configText,
                            configEditor,
                            setFormState,
                            setConfigText,
                            focusConfigSelection
                          )}
                          {control.description ? <div class="config-help">{control.description}</div> : null}
                        </div>
                      ))}
                    </div>
                  ) : null}
                  <label class="config-text-label">
                    <span>config</span>
                    <textarea
                      ref={textareaRef}
                      value={configText}
                      onInput={e => {
                        setConfigText((e.target as HTMLTextAreaElement).value);
                      }}
                      rows={18}
                      spellcheck={false}
                    />
                  </label>
                </div>
              ) : editorLoading ? (
                <div class="action-result">loading config editor...</div>
              ) : editorError ? (
                <div class="action-result err">{editorError}</div>
              ) : null}

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
