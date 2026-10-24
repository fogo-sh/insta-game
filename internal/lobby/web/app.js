const login = document.querySelector('#login');
const games = document.querySelector('#games');
const message = document.querySelector('#message');
const leave = document.querySelector('#leave');
const create = document.querySelector('#create');
const preset = document.querySelector('#preset');
const host = document.querySelector('#host');
let submission;
let key = sessionStorage.getItem('insta-game-invitation') || '';
let timer;
let loading = false;
let revision = 0;
let rendered = '';

async function api(path, method = 'GET', payload) {
  for (let attempt = 0; attempt < 3; attempt++) {
    const response = await fetch(path, { method, headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' }, body: payload ? JSON.stringify(payload) : undefined });
    if ((response.status === 429 || response.status === 503) && attempt < 2) {
      await new Promise(resolve => setTimeout(resolve, 1000 * (attempt + 1)));
      continue;
    }
    const body = await response.json().catch(() => ({}));
    if (response.status === 401) signOut();
    if (!response.ok) throw new Error(body.error || 'Service is busy; try again shortly');
    return body;
  }
}
function element(tag, text, className) {
  const node = document.createElement(tag);
  if (text !== undefined) node.textContent = text;
  if (className) node.className = className;
  return node;
}
function render(body) {
  host.hidden = false;
  host.textContent = `Host server: ${{ stopped: 'asleep', pending: 'starting', running: 'online', stopping: 'shutting down', 'shutting-down': 'shutting down', terminated: 'removed' }[body.host] || 'status unavailable'}`;
  const choices = body.presets || [];
  const previous = preset.value;
  if (JSON.stringify(choices) !== preset.dataset.choices) {
    preset.replaceChildren(...choices.map(choice => { const option = element('option', choice.name); option.value = choice.id; return option; }));
    if (choices.some(choice => choice.id === previous)) preset.value = previous;
    preset.dataset.choices = JSON.stringify(choices);
  }
  create.hidden = choices.length === 0;
  const signature = JSON.stringify([body.host, body.games.map(({ observedAt, ...game }) => game)]);
  if (signature === rendered) return;
  rendered = signature;
  const fragment = document.createDocumentFragment();
  for (const game of body.games) {
    if (game.phase === 'deleted' && !game.pending) continue;
    if (choices.some(choice => choice.id === game.id) && game.desired === 'stopped' && !game.pending) continue;
    const article = element('article');
    const heading = element('div', undefined, 'game-heading');
    const title = element('div');
    title.append(element('h2', game.name));
    if (game.game && game.game !== game.name) title.append(element('p', game.game));
    const ready = game.phase === 'running' && game.desired === 'running' && game.players !== undefined && game.address;
    const actions = element('div', undefined, 'actions');
    const requestAction = (action, label, secondary = false) => {
      const button = element('button', label, secondary ? 'secondary' : '');
      button.disabled = game.pending;
      button.addEventListener('click', async () => {
        if (action === 'remove' && !confirm(`Remove “${game.name}” and its saved data?`)) return;
        button.disabled = true;
        try { const result = await api(`/api/games/${encodeURIComponent(game.id)}/${action}`, 'POST'); message.textContent = result.message; }
        catch (error) { message.textContent = error.message; button.disabled = false; }
        await refresh();
      });
      return button;
    };
    if (game.desired === 'running' && !game.pending) actions.append(requestAction('stop', 'Stop', true));
    else if (game.desired !== 'deleted') actions.append(requestAction('start', game.pending ? ({ stop: 'Stopping…', remove: 'Removing…' }[game.action] || 'Starting…') : 'Start'));
    if (game.desired !== 'running' && !game.pending && !choices.some(choice => choice.id === game.id)) actions.append(requestAction('remove', 'Remove', true));
    heading.append(title, actions);
    let status = ({ sleeping: 'Asleep', stopped: 'Stopped', unknown: 'Checking status…', unavailable: 'Not available', failed: 'Game failed', queued: 'Starting game…', starting: 'Starting game…' })[game.phase] || game.phase;
    if (game.pending) status = body.host === 'running' ? ({ stop: 'Stopping…', remove: 'Removing…' }[game.action] || 'Starting game…') : 'Starting host…';
    if (ready && !game.pending) status = `Ready · ${game.players} player${game.players === 1 ? '' : 's'}`;
    article.append(heading, element('p', status));
    if (game.requestError || game.error) article.append(element('p', game.requestError || game.error, 'error'));
    if (ready && !game.pending) {
      const address = element('div', undefined, 'address');
      const copy = element('button', 'Copy address');
      copy.addEventListener('click', async () => {
        try { await navigator.clipboard.writeText(game.address); copy.textContent = 'Copied'; }
        catch { message.textContent = 'Select the address to copy it.'; }
      });
      address.append(element('code', game.address), copy);
      article.append(address);
    }
    fragment.append(article);
  }
  if (!fragment.childNodes.length) fragment.append(element('p', 'No servers yet. Choose a game above to start one.'));
  games.replaceChildren(fragment);
}
async function refresh() {
  if (!key || loading || document.hidden) return;
  loading = true;
  const current = revision;
  try {
    const body = await api('/api/games');
    if (current !== revision) return;
    login.hidden = true;
    leave.hidden = false;
    render(body);
  } catch (error) { message.textContent = error.message; if (key) host.textContent = 'Host server: status unavailable'; }
  finally {
    loading = false;
    clearTimeout(timer);
    if (key) timer = setTimeout(refresh, 5000);
  }
}
function signOut() {
  revision++;
  key = '';
  sessionStorage.removeItem('insta-game-invitation');
  clearTimeout(timer);
  games.replaceChildren();
  rendered = '';
  login.hidden = false;
  leave.hidden = true;
  create.hidden = true;
  host.hidden = true;
  submission = undefined;
  document.querySelector('#key').value = '';
}
login.addEventListener('submit', async (event) => {
  event.preventDefault();
  key = document.querySelector('#key').value.trim();
  sessionStorage.setItem('insta-game-invitation', key);
  message.textContent = '';
  await refresh();
});
leave.addEventListener('click', signOut);
document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(); });
refresh();

create.addEventListener('submit', async event => {
  event.preventDefault();
  const button = create.querySelector('button');
  const name = document.querySelector('#name').value.trim();
  if (!submission || submission.template !== preset.value || submission.name !== name) {
    const bytes = crypto.getRandomValues(new Uint8Array(16));
    submission = { id: `server-${Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')}`, template: preset.value, name };
  }
  button.disabled = true;
  const current = revision;
  try {
    const result = await api('/api/games', 'POST', submission);
    if (current !== revision) return;
    message.textContent = result.message;
    submission = undefined;
    document.querySelector('#name').value = '';
    await refresh();
  } catch (error) { if (current === revision) message.textContent = error.message; }
  finally { button.disabled = false; }
});
