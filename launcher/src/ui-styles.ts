export const css = `
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
  .term-fg1 { font-weight: bold; }
  .term-fg2 { color: #838887; }
  .term-fg3 { font-style: italic; }
  .term-fg4 { text-decoration: underline; }
  .term-fg30 { color: #666; }
  .term-fg31 { color: #ff7070; }
  .term-fg32 { color: #b0f986; }
  .term-fg33 { color: #c6c502; }
  .term-fg34 { color: #8db7e0; }
  .term-fg35 { color: #f271fb; }
  .term-fg36 { color: #6bf7ff; }
  .term-fg37 { color: #eee; }
  .term-fgi90 { color: #838887; }
  .term-fgi91 { color: #ff3333; }
  .term-fgi92 { color: #00ff00; }
  .term-fgi93 { color: #fffc67; }
  .term-fgi94 { color: #6871ff; }
  .term-fgi95 { color: #ff76ff; }
  .term-fgi96 { color: #60fcff; }
`;
