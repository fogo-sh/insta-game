function desiredState(server) {
  return server.pending || server.requestError ? ({ start: 'running', stop: 'stopped', remove: 'deleted' }[server.action] || server.desired) : server.desired;
}

// Desired state is intent; phase is the latest observation. Errors never imply readiness.
export function serverState(server, hostState) {
  const desired = desiredState(server);
  const { phase, pending = false } = server;
  const error = server.requestError || server.error;
  if (!pending && (phase === 'failed' || error)) return 'failed';
  if (desired === 'deleted') return phase === 'deleted' && !pending ? 'deleted' : 'deleting';
  if (desired === 'stopped') return !pending && (phase === 'stopped' || phase === 'sleeping') ? phase : 'stopping';
  if (pending && hostState !== undefined && hostState !== 'running') return 'waking';
  if (phase === 'preparing') return 'preparing';
  if (pending || phase === 'queued' || phase === 'starting') return 'starting';
  return phase || 'unknown';
}

export function stateLabel(state) {
  return ({
    waking: 'Starting host...', queued: 'Starting...', starting: 'Starting...',
    preparing: 'Downloading game...', running: 'Running', stopping: 'Stopping...',
    stopped: 'Stopped', sleeping: 'Asleep', deleting: 'Removing...', deleted: 'Removed',
    failed: 'Failed', unknown: 'Checking...', unavailable: 'Unavailable',
  })[state] || 'Checking...';
}

export function serverActions(server, preset = false) {
  const desired = desiredState(server);
  const { pending = false, phase } = server;
  const error = server.requestError || server.error;
  const failed = phase === 'failed' || Boolean(error);
  if (desired === 'deleted') return failed && !pending ? [{ action: 'remove', label: 'Retry remove' }] : [];
  const actions = [];
  if (desired === 'running') {
    actions.push({ action: 'stop', label: 'Stop' });
    if (failed && !pending) actions.push({ action: 'start', label: 'Retry' });
  } else {
    actions.push({ action: 'start', label: 'Start', disabled: pending });
    if (failed && !pending) actions.push({ action: 'stop', label: 'Retry stop' });
  }
  if (!preset) actions.push({ action: 'remove', label: 'Remove' });
  return actions;
}
