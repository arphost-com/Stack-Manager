export const updateBlockedReason = (project) => {
  if (project?.controller) return 'Use Settings > Update to update Stack Manager itself.';
  if (project?.update_policy?.effective_policy === 'no_updates') return project.update_policy?.no_updates_reason || 'Updates are disabled for this project.';
  if (!project?.update_status?.checked) return 'No update check has run yet.';
  if (!project?.update_status?.available) return 'No image updates were available at the last check.';
  return '';
};

export const canRunImageUpdate = (project) => updateBlockedReason(project) === '';

export const updateStatusLabel = (project) => {
  const status = project?.update_status || {};
  if (project?.controller) return 'self-update only';
  if (project?.update_policy?.effective_policy === 'no_updates') return 'disabled';
  if (!status.checked) return 'not checked';
  if (status.available) return `${status.count || 1} available`;
  if (status.error) return 'check warning';
  return 'current';
};

export const updateStatusTone = (project) => {
  const status = project?.update_status || {};
  if (project?.controller) return 'blue';
  if (project?.update_policy?.effective_policy === 'no_updates') return 'amber';
  if (!status.checked) return 'gray';
  if (status.available) return 'green';
  if (status.error) return 'amber';
  return 'gray';
};
