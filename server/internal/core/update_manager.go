package core

import (
	"context"
	"sync"
	"time"
)

type UpdateCheckStore interface {
	SaveProjectUpdateStatus(context.Context, string, ProjectUpdateStatus) error
	ResolveUpdatePolicy(Project) ProjectUpdatePolicy
	ScheduledProjectNames(context.Context) map[string]bool
}

type UpdateCheckManager struct {
	engine *Engine
	store  UpdateCheckStore
	stop   chan struct{}
	mu     sync.Mutex
}

func NewUpdateCheckManager(engine *Engine, store UpdateCheckStore) *UpdateCheckManager {
	return &UpdateCheckManager{
		engine: engine,
		store:  store,
		stop:   make(chan struct{}),
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
	m.mu.Lock()
	defer m.mu.Unlock()

	summary := ProjectUpdateStatus{Checked: true}
	if m == nil || m.engine == nil || m.store == nil {
		summary.Error = "update checker is not configured"
		return summary
	}
	projects, err := m.engine.DiscoverProjects()
	if err != nil {
		summary.Error = err.Error()
		return summary
	}
	// Skip projects that have an enabled scheduled update — the
	// scheduler will check for updates when it runs, so burning
	// Docker Hub pull budget on a daily manifest inspect is waste.
	scheduledProjects := m.store.ScheduledProjectNames(ctx)

	for i := range projects {
		project := &projects[i]
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
		_ = m.store.SaveProjectUpdateStatus(ctx, project.Name, status)
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
			summary.CheckedAt = status.CheckedAt
		}
		summary.Images = append(summary.Images, status.Images...)
	}
	next := nextLocalMidnight(time.Now())
	summary.NextCheckAt = &next
	return summary
}

func nextLocalMidnight(now time.Time) time.Time {
	local := now.Local()
	next := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, local.Location())
	return next.UTC()
}
