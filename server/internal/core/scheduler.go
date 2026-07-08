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
	if schedule.Cadence == "interval" && schedule.IntervalMinutes < 1 {
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

// NextScheduleRunExported is the exported version for use by the storage
// layer when computing the initial next_run_at at save time.
func NextScheduleRunExported(schedule UpdateSchedule, after time.Time) time.Time {
	return nextScheduleRun(schedule, after)
}

func nextScheduleRun(schedule UpdateSchedule, after time.Time) time.Time {
	switch schedule.Cadence {
	case "daily":
		return nextDaily(schedule.TimeOfDay, after)
	case "weekly":
		return nextWeekly(schedule.DayOfWeek, schedule.TimeOfDay, after)
	case "monthly":
		return nextMonthly(schedule.DayOfMonth, schedule.TimeOfDay, after)
	default:
		return nextInterval(schedule.IntervalMinutes, schedule.NextRunAt, after)
	}
}

func nextInterval(intervalMinutes int, nextRunAt, after time.Time) time.Time {
	step := time.Duration(intervalMinutes) * time.Minute
	if step <= 0 {
		step = 24 * time.Hour
	}
	next := nextRunAt
	if next.IsZero() || !next.After(after) {
		next = after.Add(step)
	}
	for !next.After(after) {
		next = next.Add(step)
	}
	return next.UTC()
}

func parseTimeOfDay(tod string) (int, int) {
	parts := strings.SplitN(tod, ":", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	if h < 0 || h > 23 {
		h = 0
	}
	if m < 0 || m > 59 {
		m = 0
	}
	return h, m
}

func nextDaily(timeOfDay string, after time.Time) time.Time {
	h, m := parseTimeOfDay(timeOfDay)
	candidate := time.Date(after.Year(), after.Month(), after.Day(), h, m, 0, 0, time.UTC)
	if !candidate.After(after) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

func nextWeekly(dayOfWeek int, timeOfDay string, after time.Time) time.Time {
	h, m := parseTimeOfDay(timeOfDay)
	candidate := time.Date(after.Year(), after.Month(), after.Day(), h, m, 0, 0, time.UTC)
	target := time.Weekday(dayOfWeek)
	daysAhead := int(target - candidate.Weekday())
	if daysAhead < 0 {
		daysAhead += 7
	}
	candidate = candidate.AddDate(0, 0, daysAhead)
	if !candidate.After(after) {
		candidate = candidate.AddDate(0, 0, 7)
	}
	return candidate
}

func nextMonthly(dayOfMonth int, timeOfDay string, after time.Time) time.Time {
	h, m := parseTimeOfDay(timeOfDay)
	if dayOfMonth < 1 {
		dayOfMonth = 1
	}
	candidate := clampMonthDay(after.Year(), after.Month(), dayOfMonth, h, m)
	if !candidate.After(after) {
		nextMonth := after.Month() + 1
		nextYear := after.Year()
		if nextMonth > 12 {
			nextMonth = 1
			nextYear++
		}
		candidate = clampMonthDay(nextYear, nextMonth, dayOfMonth, h, m)
	}
	return candidate
}

func clampMonthDay(year int, month time.Month, day, hour, min int) time.Time {
	last := daysInMonth(year, month)
	if day > last {
		day = last
	}
	return time.Date(year, month, day, hour, min, 0, 0, time.UTC)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
