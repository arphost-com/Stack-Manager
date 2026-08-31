export const projectState = (project) => {
  const state = String(project?.state || '').trim().toLowerCase();
  if (state) return state;
  return project?.running ? 'running' : 'stopped';
};

export const stateTone = (state) => {
  switch (String(state || '').trim().toLowerCase()) {
    case 'running': return 'green';
    case 'restarting':
    case 'dead': return 'red';
    case 'paused':
    case 'removing': return 'amber';
    case 'created': return 'blue';
    default: return 'gray';
  }
};

export const projectStateTone = (project) => stateTone(projectState(project));
