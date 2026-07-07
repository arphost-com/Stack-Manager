package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ScheduleStore interface {
	ListDueSchedules(context.Context, time.Time) ([]UpdateSchedule, error)
	MarkScheduleDispatched(context.Context, int64, time.Time, string, string, string) error
	GetAgent(context.Context, int64) (*ComposeAgent, error)
	ResolveUpdatePolicy(Project) ProjectUpdatePolicy
}

type ScheduleManager struct {
	engine *Engine
	jobs   *JobManager
	store  ScheduleStore
	client *http.Client
	stop   chan struct{}
}

func NewScheduleManager(engine *Engine, jobs *JobManager, store ScheduleStore) *ScheduleManager {
	return &ScheduleManager{
		engine: engine,
		jobs:   jobs,
		store:  store,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		stop: make(chan struct{}),
	}
}

func (m *ScheduleManager) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		m.tick(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stop:
				return
			case <-ticker.C:
				m.tick(ctx)
			}
		}
	}()
}

func (m *ScheduleManager) Stop() {
	close(m.stop)
}

func (m *ScheduleManager) RunNow(ctx context.Context, schedule UpdateSchedule) error {
	return m.runSchedule(ctx, schedule)
}

func (m *ScheduleManager) tick(ctx context.Context) {
	if m == nil || m.store == nil {
		return
	}
	schedules, err := m.store.ListDueSchedules(ctx, time.Now().UTC())
	if err != nil {
		return
	}
	for _, schedule := range schedules {
		if err := m.runSchedule(ctx, schedule); err != nil {
			next := nextScheduleRun(schedule, time.Now().UTC())
			_ = m.store.MarkScheduleDispatched(ctx, schedule.ID, next, "", "failed", err.Error())
		}
	}
}

func (m *ScheduleManager) runSchedule(ctx context.Context, schedule UpdateSchedule) error {
	if schedule.IntervalMinutes < 1 {
		return fmt.Errorf("schedule %d has invalid interval", schedule.ID)
	}
	action := strings.ToLower(strings.TrimSpace(schedule.Action))
	if action == "" {
		action = "update"
	}
	if !ValidJobAction(action) {
		return fmt.Errorf("invalid scheduled action: %s", action)
	}
	next := nextScheduleRun(schedule, time.Now().UTC())
	if schedule.AgentID != nil {
		jobID, err := m.runAgentSchedule(ctx, schedule, action)
		if err != nil {
			_ = m.store.MarkScheduleDispatched(ctx, schedule.ID, next, "", "failed", err.Error())
			return err
		}
		return m.store.MarkScheduleDispatched(ctx, schedule.ID, next, jobID, "started", "")
	}

	project, err := m.engine.GetProject(schedule.Project)
	if err != nil {
		_ = m.store.MarkScheduleDispatched(ctx, schedule.ID, next, "", "failed", err.Error())
		return err
	}
	if action == "update" {
		policy := m.store.ResolveUpdatePolicy(*project)
		if policy.EffectivePolicy == UpdatePolicyNoUpdates {
			reason := policy.NoUpdatesReason
			if reason == "" {
				reason = "updates disabled"
			}
			job, err := m.jobs.StartSkipped(project, action, "scheduled update skipped: "+reason+"\n")
			if err != nil {
				_ = m.store.MarkScheduleDispatched(ctx, schedule.ID, next, "", "failed", err.Error())
				return err
			}
			return m.store.MarkScheduleDispatched(ctx, schedule.ID, next, job.ID, "skipped", "")
		}
	}
	job, err := m.jobs.Start(m.engine, project, action, schedule.TimeoutSeconds)
	if err != nil {
		_ = m.store.MarkScheduleDispatched(ctx, schedule.ID, next, "", "failed", err.Error())
		return err
	}
	return m.store.MarkScheduleDispatched(ctx, schedule.ID, next, job.ID, "started", "")
}

func (m *ScheduleManager) runAgentSchedule(ctx context.Context, schedule UpdateSchedule, action string) (string, error) {
	agent, err := m.store.GetAgent(ctx, *schedule.AgentID)
	if err != nil {
		return "", err
	}
	if !agent.Enabled {
		return "", fmt.Errorf("agent %s is disabled", agent.Name)
	}
	if agent.BaseURL == "" {
		return "", fmt.Errorf("agent %s uses outbound check-in mode; scheduled remote actions require a command queue", agent.Name)
	}
	base, err := url.Parse(agent.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid agent URL")
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", fmt.Errorf("agent URL must use http or https")
	}
	escapedProject := url.PathEscape(schedule.Project)
	path := strings.TrimRight(base.String(), "/") + "/agent/v1/projects/" + escapedProject + "/jobs/" + url.PathEscape(action)
	if schedule.TimeoutSeconds > 0 {
		path += "?timeout=" + strconv.Itoa(schedule.TimeoutSeconds)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(nil))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+agent.Token)
	res, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var envelope struct {
		Status string     `json:"status"`
		Error  string     `json:"error"`
		Data   *ActionJob `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || envelope.Status == "error" {
		if envelope.Error == "" {
			envelope.Error = res.Status
		}
		return "", errors.New(envelope.Error)
	}
	if envelope.Data == nil || envelope.Data.ID == "" {
		return "", fmt.Errorf("agent did not return a job id")
	}
	return envelope.Data.ID, nil
}

func nextScheduleRun(schedule UpdateSchedule, after time.Time) time.Time {
	next := schedule.NextRunAt
	step := time.Duration(schedule.IntervalMinutes) * time.Minute
	if step <= 0 {
		step = 24 * time.Hour
	}
	if next.IsZero() || !next.After(after) {
		next = after.Add(step)
	}
	for !next.After(after) {
		next = next.Add(step)
	}
	return next.UTC()
}
