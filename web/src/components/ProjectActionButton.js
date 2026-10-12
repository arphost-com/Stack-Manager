import { createElement } from 'react';
import { canRunProjectAction, projectActionBlockedReason } from '../utils/updateEligibility.js';

// Shared by project details, dashboard rows, and bulk actions so their manual
// Pull controls cannot acquire different update-check prerequisites.
export default function ProjectActionButton({ project, projects, action, disabled = false, disabledReason = '', title, children, ...props }) {
  const reason = project ? projectActionBlockedReason(project, action) : '';
  const noTargets = projects ? !projects.some(target => canRunProjectAction(target, action)) : false;
  return createElement('button', {
    ...props,
    disabled: disabled || !!reason || noTargets,
    title: (disabled && disabledReason) || reason || title,
  }, children);
}
