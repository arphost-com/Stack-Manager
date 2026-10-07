package core

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

const updateCheckCooldown = 5 * time.Minute

type UpdateCheckStore interface {
	SaveProjectUpdateStatus(context.Context, string, ProjectUpdateStatus) error
	ProjectUpdateStatus(context.Context, string) (ProjectUpdateStatus, error)
	ResolveUpdatePolicy(Project) ProjectUpdatePolicy
	ScheduledProjectNames(context.Context) map[string]bool
}

type UpdateCheckManager struct {
	engine      *Engine
	store       UpdateCheckStore
	stop        chan struct{}
	mu          sync.Mutex
	lastRunAt   time.Time
	lastSummary ProjectUpdateStatus
	now         func() time.Time
}

func NewUpdateCheckManager(engine *Engine, store UpdateCheckStore) *UpdateCheckManager {
	return &UpdateCheckManager{
		engine: engine,
		store:  store,
		stop:   make(chan struct{}),
		now:    time.Now,
	}
}

func (m *UpdateCheckManager) Start(ctx context.Context) {
	if m == nil || m.engine == nil || m.store == nil {
		return
	}
	go func() {
		startup := time.NewTimer(15 * time.Second)
		for {
			timer := time.NewTimer(time.Until(nextLocalMidnight(time.Now())))
			select {
			case <-ctx.Done():
				timer.Stop()
				startup.Stop()
				return
			case <-m.stop:
				timer.Stop()
				startup.Stop()
				return
			case <-startup.C:
				timer.Stop()
				m.Run(ctx)
			case <-timer.C:
				m.Run(ctx)
			}
		}
	}()
}

func (m *UpdateCheckManager) Stop() {
	if m == nil {
		return
	}
	close(m.stop)
}

func (m *UpdateCheckManager) Run(ctx context.Context) ProjectUpdateStatus {
	if m == nil {
		return ProjectUpdateStatus{Checked: true, Error: "update checker is not configured"}
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.currentTime()
	if cached, ok := m.cooldownResult(now); ok {
		return cached
	}

	summary := ProjectUpdateStatus{Checked: true}
	if m.engine == nil || m.store == nil {
		summary.Error = "update checker is not configured"
		m.rememberResult(m.currentTime(), summary)
		return summary
	}
	projects, err := m.engine.DiscoverProjects()
	if err != nil {
		summary.Error = err.Error()
		m.rememberResult(m.currentTime(), summary)
		return summary
	}
	// Skip projects that have an enabled scheduled update — the
	// scheduler will check for updates when it runs, so burning
	// Docker Hub pull budget on a daily manifest inspect is waste.
	scheduledProjects := m.store.ScheduledProjectNames(ctx)

	for i := range projects {
		project := &projects[i]
		if m.engine.IsControllerProject(project) {
			// The controller has a dedicated Settings > Update workflow. Clear
			// any stale ordinary-stack notice left by an older release.
			_ = m.store.SaveProjectUpdateStatus(ctx, project.Name, ProjectUpdateStatus{})
			continue
		}
		if project.Inactive {
			continue
		}
		if scheduledProjects[project.Name] {
			continue
		}
		policy := m.store.ResolveUpdatePolicy(*project)
		if policy.EffectivePolicy == UpdatePolicyNoUpdates {
			continue
		}
		status := m.engine.CheckProjectUpdates(ctx, project)
		if status.Error != "" {
			if previous, err := m.store.ProjectUpdateStatus(ctx, project.Name); err == nil {
				status = preservePreviousUpdateResults(previous, status)
			}
		}
		_ = m.store.SaveProjectUpdateStatus(ctx, project.Name, status)
		appendUpdateSummary(&summary, status)
		if IsRegistryRateLimitError(status.Error) {
			for j := i + 1; j < len(projects); j++ {
				cachedProject := &projects[j]
				if cachedProject.Inactive || m.engine.IsControllerProject(cachedProject) || scheduledProjects[cachedProject.Name] {
					continue
				}
				if policy := m.store.ResolveUpdatePolicy(*cachedProject); policy.EffectivePolicy == UpdatePolicyNoUpdates {
					continue
				}
				if previous, err := m.store.ProjectUpdateStatus(ctx, cachedProject.Name); err == nil {
					appendUpdateSummary(&summary, previous)
				}
			}
			summary.Message = "Registry rate limit reached. Stack Manager stopped additional checks and kept the last known update results."
			summary.Cached = true
			summary.RetryAfterSeconds = int(updateCheckCooldown.Seconds())
			break
		}
	}

	completedAt := m.currentTime()
	if summary.RetryAfterSeconds > 0 {
		allowed := completedAt.Add(updateCheckCooldown)
		summary.RefreshAllowedAt = &allowed
	}
	next := nextLocalMidnight(completedAt)
	summary.NextCheckAt = &next
	m.rememberResult(completedAt, summary)
	return summary
}

func (m *UpdateCheckManager) currentTime() time.Time {
	if m != nil && m.now != nil {
		return m.now().UTC()
	}
	return time.Now().UTC()
}

func (m *UpdateCheckManager) cooldownResult(now time.Time) (ProjectUpdateStatus, bool) {
	if m.lastRunAt.IsZero() || now.Before(m.lastRunAt) || now.Sub(m.lastRunAt) >= updateCheckCooldown {
		return ProjectUpdateStatus{}, false
	}
	remaining := updateCheckCooldown - now.Sub(m.lastRunAt)
	status := m.lastSummary
	status.Images = append([]ImageUpdateCheck(nil), m.lastSummary.Images...)
	status.Cached = true
	status.RetryAfterSeconds = int(math.Ceil(remaining.Seconds()))
	allowed := m.lastRunAt.Add(updateCheckCooldown)
	status.RefreshAllowedAt = &allowed
	status.Message = fmt.Sprintf("Updates were checked recently. Showing cached results; another registry refresh is available in %s.", friendlyCooldown(remaining))
	return status, true
}

func (m *UpdateCheckManager) rememberResult(now time.Time, status ProjectUpdateStatus) {
	m.lastRunAt = now
	m.lastSummary = status
	m.lastSummary.Images = append([]ImageUpdateCheck(nil), status.Images...)
}

func friendlyCooldown(remaining time.Duration) string {
	remaining = remaining.Round(time.Second)
	if remaining < time.Second {
		return "a moment"
	}
	return remaining.String()
}

func appendUpdateSummary(summary *ProjectUpdateStatus, status ProjectUpdateStatus) {
	if summary == nil || !status.Checked {
		return
	}
	summary.RegistryImages += status.RegistryImages
	summary.SkippedServices += status.SkippedServices
	if status.Available {
		summary.Available = true
		summary.Count += status.Count
	}
	if status.Error != "" && summary.Error == "" {
		summary.Error = status.Error
	}
	if status.CheckedAt != nil && (summary.CheckedAt == nil || status.CheckedAt.After(*summary.CheckedAt)) {
		t := status.CheckedAt.UTC()
		summary.CheckedAt = &t
	}
	summary.Images = append(summary.Images, status.Images...)
}

func preservePreviousUpdateResults(previous, current ProjectUpdateStatus) ProjectUpdateStatus {
	if current.Error == "" || !previous.Checked || len(previous.Images) == 0 {
		return current
	}
	previousByImage := make(map[string]ImageUpdateCheck, len(previous.Images))
	for _, image := range previous.Images {
		previousByImage[image.Service+"\x00"+image.Image] = image
	}
	if len(current.Images) == 0 {
		current.Images = append([]ImageUpdateCheck(nil), previous.Images...)
		for i := range current.Images {
			current.Images[i].Error = ""
		}
		current.Images[0].Error = current.Error
		current.RegistryImages = previous.RegistryImages
		current.SkippedServices = previous.SkippedServices
	} else {
		seen := make(map[string]bool, len(current.Images))
		for i := range current.Images {
			failed := current.Images[i]
			key := failed.Service + "\x00" + failed.Image
			seen[key] = true
			if failed.Error == "" {
				continue
			}
			if cached, ok := previousByImage[key]; ok {
				cached.Project = failed.Project
				cached.Service = failed.Service
				cached.Image = failed.Image
				cached.Error = failed.Error
				current.Images[i] = cached
			}
		}
		if IsRegistryRateLimitError(current.Error) {
			for _, cached := range previous.Images {
				if seen[cached.Service+"\x00"+cached.Image] {
					continue
				}
				cached.Error = ""
				current.Images = append(current.Images, cached)
			}
			current.RegistryImages = previous.RegistryImages
			current.SkippedServices = previous.SkippedServices
		}
	}
	current.Checked = true
	current.Cached = true
	current.Available = false
	current.Count = 0
	for _, image := range current.Images {
		if image.UpdateAvailable {
			current.Available = true
			current.Count++
		}
	}
	return current
}

func nextLocalMidnight(now time.Time) time.Time {
	local := now.Local()
	next := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, local.Location())
	return next.UTC()
}
