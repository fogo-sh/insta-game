const login = document.querySelector('#login');
const games = document.querySelector('#games');
const message = document.querySelector('#message');
const leave = document.querySelector('#leave');
let key = sessionStorage.getItem('insta-game-invitation') || '';
let timer;
let loading = false;
let revision = 0;
let rendered = '';

async function api(path, method = 'GET') {
  for (let attempt = 0; attempt < 3; attempt++) {
    const response = await fetch(path, { method, headers: { Authorization: `Bearer ${key}` } });
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
  const signature = JSON.stringify([body.host, body.games.map(({ observedAt, ...game }) => game)]);
  if (signature === rendered) return;
  rendered = signature;
  const fragment = document.createDocumentFragment();
  for (const game of body.games) {
    const article = element('article');
    const heading = element('div', undefined, 'game-heading');
    const title = element('div');
    title.append(element('h2', game.name));
    if (game.game && game.game !== game.name) title.append(element('p', game.game));
    const ready = game.phase === 'running' && game.desired === 'running' && game.players !== undefined && game.address;
    const start = element('button', game.pending ? 'Starting…' : ready ? 'Running' : 'Start');
    start.disabled = game.pending || !!ready || game.phase === 'unavailable';
    start.addEventListener('click', async () => {
      start.disabled = true;
      try {
        const result = await api(`/api/games/${encodeURIComponent(game.id)}/start`, 'POST');
        message.textContent = result.message;
      } catch (error) {
        message.textContent = error.message;
        start.disabled = false;
      }
      await refresh();
    });
    heading.append(title, start);
    let status = ({ sleeping: 'Asleep', stopped: 'Stopped', unknown: 'Checking status…', unavailable: 'Not available', failed: 'Game failed', queued: 'Starting game…', starting: 'Starting game…' })[game.phase] || game.phase;
    if (game.pending) status = body.host === 'running' ? 'Starting game…' : 'Starting host…';
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
  } catch (error) { message.textContent = error.message; }
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
