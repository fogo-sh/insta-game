const login = document.querySelector('#login');
const games = document.querySelector('#games');
const message = document.querySelector('#message');
const leave = document.querySelector('#leave');
const create = document.querySelector('#create');
const preset = document.querySelector('#preset');
const host = document.querySelector('#host');
const hostDetails = document.querySelector('#host-details');
let submission;
let choices = [];
let autoPlace = false;
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
    if (!response.ok) throw new Error(body.error || 'Try again shortly.');
    return body;
  }
}
function element(tag, text, className) {
  const node = document.createElement(tag);
  if (text !== undefined) node.textContent = text;
  if (className) node.className = className;
  return node;
}
function gameTitle(name) {
  return ({ 'FTEQW (Quake 1)': 'Quake I', 'q2repro (Quake 2)': 'Quake II', 'ioquake3 (Quake 3 Arena)': 'Quake III Arena', 'UT99 GOTY': 'Unreal Tournament 99' })[name] || name || 'Other games';
}
function route(hostId, path) {
  return hostId ? `/api/hosts/${encodeURIComponent(hostId)}${path}` : `/api${path}`;
}
function identity(hostId, id) { return `${hostId || ''}/${id}`; }
function serverRow(game, member, isPreset) {
  const article = element('article');
  const title = element('div', undefined, 'server-main');
  title.append(element('h3', game.name));
  if (member.name) title.append(element('p', member.name, 'host-meta'));
  const ready = game.phase === 'running' && game.desired === 'running' && game.players !== undefined && game.address;
  const connection = element('div', undefined, 'connection');
  let status = ({ sleeping: 'Asleep', stopped: 'Stopped', unknown: 'Checking...', unavailable: 'Unavailable', failed: 'Failed', queued: 'Starting...', starting: 'Starting...' })[game.phase] || game.phase;
  if (game.pending) status = member.state === 'running' ? ({ stop: 'Stopping...', remove: 'Removing...' }[game.action] || 'Starting...') : 'Starting host...';
  if (ready && !game.pending) {
    status = `${game.players} player${game.players === 1 ? '' : 's'}`;
    const copy = element('button', undefined, 'address');
    copy.type = 'button';
    copy.title = 'Copy address';
    copy.setAttribute('aria-label', `Copy ${game.name} address`);
    copy.append(element('code', game.address));
    copy.addEventListener('click', async () => {
      try { await navigator.clipboard.writeText(game.address); message.textContent = 'Address copied.'; }
      catch { message.textContent = 'Select the address to copy it.'; }
    });
    connection.append(copy);
  }
  connection.append(element('p', status, 'server-status'));
  const actions = element('div', undefined, 'actions');
  const actionButton = (action, label) => {
    const button = element('button', label, 'quiet');
    button.type = 'button';
    button.disabled = game.pending;
    button.addEventListener('click', async () => {
      if (action === 'remove' && !confirm(`Remove "${game.name}" and its saved data?`)) return;
      button.disabled = true;
      try { const result = await api(route(game.hostId, `/games/${encodeURIComponent(game.id)}/${action}`), 'POST'); message.textContent = result.message; }
      catch (error) { message.textContent = error.message; button.disabled = false; }
      await refresh();
    });
    return button;
  };
  if (game.desired === 'running' && !game.pending) actions.append(actionButton('stop', 'Stop'));
  else if (game.desired !== 'deleted') actions.append(actionButton('start', game.pending ? ({ stop: 'Stopping...', remove: 'Removing...' }[game.action] || 'Starting...') : 'Start'));
  if (game.desired !== 'running' && !game.pending && !isPreset) actions.append(actionButton('remove', 'Remove'));
  article.append(title, connection, actions);
  if (game.requestError || game.error) article.append(element('p', game.requestError || game.error, 'error'));
  return article;
}
function render(body) {
  const hosts = body.hosts || [{ id: '', name: '', state: body.host }];
  const hostById = new Map(hosts.map(h => [h.id, h]));
  autoPlace = Array.isArray(body.choices);
  if (autoPlace) choices = body.choices;
  else {
    const byGame = new Map();
    for (const p of body.presets || []) if (!byGame.has(p.game || p.name)) byGame.set(p.game || p.name, { id: p.id, name: p.game || p.name });
    choices = [...byGame.values()];
  }
  choices.sort((a, b) => gameTitle(a.name).localeCompare(gameTitle(b.name)));
  const previous = preset.value;
  const signatureChoices = JSON.stringify(choices);
  if (signatureChoices !== preset.dataset.choices) {
    preset.replaceChildren(...choices.map(choice => { const option = element('option', gameTitle(choice.name)); option.value = choice.id; return option; }));
    if (choices.some(choice => choice.id === previous)) preset.value = previous;
    preset.dataset.choices = signatureChoices;
  }
  create.hidden = choices.length === 0;
  const presetIds = new Set((body.presets || []).map(p => identity(p.hostId, p.id)));
  const visible = body.games.filter(g => !(g.phase === 'deleted' && !g.pending) && !(presetIds.has(identity(g.hostId, g.id)) && g.desired === 'stopped' && !g.pending));
  host.hidden = false;
  host.textContent = `${visible.length} server${visible.length === 1 ? '' : 's'}${hosts.some(h => h.error) ? ' (some statuses unavailable)' : ''}`;
  const signature = JSON.stringify([hosts, visible.map(({ observedAt, ...g }) => g)]);
  if (signature === rendered) return;
  rendered = signature;
  hostDetails.hidden = false;
  const statuses = document.querySelector('#host-statuses');
  statuses.replaceChildren(...hosts.map(h => element('p', `${h.name || 'Host'}: ${h.error || ({ running: 'online', stopped: 'asleep', pending: 'starting', stopping: 'stopping', 'shutting-down': 'stopping', terminated: 'removed' }[h.state] || 'status unavailable')}`)));
  const byGame = new Map();
  for (const game of visible) {
    if (!byGame.has(game.game)) byGame.set(game.game, []);
    byGame.get(game.game).push(game);
  }
  const fragment = document.createDocumentFragment();
  for (const [name, servers] of [...byGame].sort(([a], [b]) => gameTitle(a).localeCompare(gameTitle(b)))) {
    const group = element('section', undefined, 'game-group');
    const heading = element('div', undefined, 'group-heading');
    heading.append(element('h2', gameTitle(name)), element('span', String(servers.length), 'group-count'));
    group.append(heading);
    servers.sort((a, b) => a.name.localeCompare(b.name) || (a.hostId || '').localeCompare(b.hostId || ''));
    for (const game of servers) group.append(serverRow(game, hostById.get(game.hostId || '') || {}, presetIds.has(identity(game.hostId, game.id))));
    fragment.append(group);
  }
  if (!visible.length) fragment.append(element('p', 'No servers yet.', 'muted'));
  games.replaceChildren(fragment);
}
async function refresh() {
  if (!key || loading || document.hidden) return;
  loading = true;
  const current = revision;
  games.setAttribute('aria-busy', 'true');
  try {
    const body = await api('/api/games');
    if (current !== revision) return;
    login.hidden = true;
    leave.hidden = false;
    render(body);
  } catch (error) { message.textContent = error.message; }
  finally {
    loading = false;
    games.setAttribute('aria-busy', 'false');
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
  hostDetails.hidden = true;
  submission = undefined;
  document.querySelector('#key').value = '';
}
login.addEventListener('submit', async event => {
  event.preventDefault();
  key = document.querySelector('#key').value.trim();
  sessionStorage.setItem('insta-game-invitation', key);
  message.textContent = 'Loading servers...';
  await refresh();
  if (!login.hidden) return;
  message.textContent = '';
});
leave.addEventListener('click', signOut);
document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(); });
refresh();
create.addEventListener('submit', async event => {
  event.preventDefault();
  const button = create.querySelector('button');
  const enteredName = document.querySelector('#name').value.trim();
  const choice = choices.find(c => c.id === preset.value);
  if (!choice) return;
  const name = enteredName || gameTitle(choice.name);
  if (!submission || submission.choice !== choice.id || submission.name !== name) {
    const bytes = crypto.getRandomValues(new Uint8Array(16));
    submission = { id: `server-${Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('')}`, choice: choice.id, name };
  }
  button.disabled = true;
  const current = revision;
  try {
    const field = autoPlace ? 'game' : 'template';
    const result = await api('/api/games', 'POST', { id: submission.id, [field]: submission.choice, name: submission.name });
    if (current !== revision) return;
    message.textContent = result.message;
    submission = undefined;
    document.querySelector('#name').value = '';
    await refresh();
  } catch (error) { if (current === revision) message.textContent = error.message; }
  finally { button.disabled = false; }
});
