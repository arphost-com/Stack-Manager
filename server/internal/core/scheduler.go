package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// schedulerLocation returns the server's configured timezone (TZ env, set from
// the TZ setting) so time-of-day schedules fire in local time rather than UTC.
// Falls back to UTC on an empty or unknown zone. The tz database is embedded in
// the binary (see the time/tzdata import in main) so this works in any container.
func schedulerLocation() *time.Location {
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.UTC
}

type ScheduleStore interface {
	ListDueSchedules(context.Context, time.Time) ([]UpdateSchedule, error)
	MarkScheduleDispatched(context.Context, int64, time.Time, string, string, string) error
	CompleteScheduleRun(context.Context, string, string, string, string, time.Time, string) error
	GetAgent(context.Context, int64) (*ComposeAgent, error)
	ResolveUpdatePolicy(Project) ProjectUpdatePolicy
	EnqueueAgentCommand(context.Context, int64, AgentCommandRequest) (*AgentCommand, error)
}

type ScheduleManager struct {
	engine *Engine
	jobs   *JobManager
	store  ScheduleStore
	client *http.Client
	stop   chan struct{}
}

func NewScheduleManager(engine *Engine, jobs *JobManager, store ScheduleStore) *ScheduleManager {
	manager := &ScheduleManager{
		engine: engine,
		jobs:   jobs,
		store:  store,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{ // nosemgrep: problem-based-packs.insecure-transport.go-stdlib.bypass-tls-verification.bypass-tls-verification
					InsecureSkipVerify: true,
					MinVersion:         tls.VersionTLS13,
				},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		stop: make(chan struct{}),
	}
	if jobs != nil && store != nil {
		jobs.AddCompletionHandler(func(job *ActionJob) {
			_ = store.CompleteScheduleRun(
				context.Background(),
				job.ID,
				job.Status,
				job.Output,
				job.Error,
				job.EndedAt,
				job.Duration,
			)
		})
	}
	return manager
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
		_ = m.runSchedule(ctx, schedule)
	}
}

func (m *ScheduleManager) runSchedule(ctx context.Context, schedule UpdateSchedule) error {
	next := nextScheduleRun(schedule, time.Now().UTC())
	fail := func(err error) error {
		if markErr := m.store.MarkScheduleDispatched(ctx, schedule.ID, next, "", "failed", err.Error()); markErr != nil {
			return fmt.Errorf("%w (record schedule failure: %v)", err, markErr)
		}
		return err
	}
	if schedule.Cadence == "interval" && schedule.IntervalMinutes < 1 {
		return fail(fmt.Errorf("schedule %d has invalid interval", schedule.ID))
	}
	action := strings.ToLower(strings.TrimSpace(schedule.Action))
	if action == "" {
		action = "update"
	}
	if !ValidJobAction(action) {
		return fail(fmt.Errorf("invalid scheduled action: %s", action))
	}
	if schedule.AgentID != nil {
		jobID, pollRemote, err := m.runAgentSchedule(ctx, schedule, action)
		if err != nil {
			return fail(err)
		}
		if err := m.store.MarkScheduleDispatched(ctx, schedule.ID, next, jobID, "started", ""); err != nil {
			return err
		}
		if pollRemote {
			go m.watchAgentJob(schedule, jobID)
		}
		return nil
	}

	project, err := m.engine.GetProject(schedule.Project)
	if err != nil {
		return fail(err)
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
				return fail(err)
			}
			return m.store.MarkScheduleDispatched(ctx, schedule.ID, next, job.ID, "skipped", "")
		}
	}
	job, err := m.jobs.Start(m.engine, project, action, schedule.TimeoutSeconds)
	if err != nil {
		return fail(err)
	}
	if err := m.store.MarkScheduleDispatched(ctx, schedule.ID, next, job.ID, "started", ""); err != nil {
		return err
	}
	// Very short jobs can finish before the dispatch row is inserted. Reconcile
	// once after insertion to close that race; normal completions use the hook.
	if current, ok := m.jobs.Get(job.ID); ok && current.Status != "running" {
		_ = m.store.CompleteScheduleRun(ctx, current.ID, current.Status, current.Output, current.Error, current.EndedAt, current.Duration)
	}
	return nil
}

func (m *ScheduleManager) runAgentSchedule(ctx context.Context, schedule UpdateSchedule, action string) (string, bool, error) {
	agent, err := m.store.GetAgent(ctx, *schedule.AgentID)
	if err != nil {
		return "", false, err
	}
	if !agent.Enabled {
		return "", false, fmt.Errorf("agent %s is disabled", agent.Name)
	}
	if agent.BaseURL == "" || agent.Mode == "callback" {
		// Callback agents can't be driven live — queue the action so the agent
		// runs it on its next check-in (the same command queue the UI uses).
		cmd, err := m.store.EnqueueAgentCommand(ctx, agent.ID, AgentCommandRequest{
			Project: schedule.Project,
			Action:  action,
			Params:  fmt.Sprintf(`{"timeout":%d}`, schedule.TimeoutSeconds),
		})
		if err != nil {
			return "", false, err
		}
		return fmt.Sprintf("agent-cmd-%d", cmd.ID), false, nil
	}
	base, err := url.Parse(agent.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", false, fmt.Errorf("invalid agent URL")
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", false, fmt.Errorf("agent URL must use http or https")
	}
	escapedProject := url.PathEscape(schedule.Project)
	path := strings.TrimRight(base.String(), "/") + "/agent/v1/projects/" + escapedProject + "/jobs/" + url.PathEscape(action)
	if schedule.TimeoutSeconds > 0 {
		path += "?timeout=" + strconv.Itoa(schedule.TimeoutSeconds)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(nil))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+agent.Token)
	res, err := m.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer res.Body.Close()
	var envelope struct {
		Status string     `json:"status"`
		Error  string     `json:"error"`
		Data   *ActionJob `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return "", false, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || envelope.Status == "error" {
		if envelope.Error == "" {
			envelope.Error = res.Status
		}
		return "", false, errors.New(envelope.Error)
	}
	if envelope.Data == nil || envelope.Data.ID == "" {
		return "", false, fmt.Errorf("agent did not return a job id")
	}
	return envelope.Data.ID, true, nil
}

func (m *ScheduleManager) watchAgentJob(schedule UpdateSchedule, jobID string) {
	timeout := time.Duration(schedule.TimeoutSeconds)*time.Second + 10*time.Minute
	if timeout < 15*time.Minute {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	agent, err := m.store.GetAgent(ctx, *schedule.AgentID)
	if err != nil {
		_ = m.store.CompleteScheduleRun(context.Background(), jobID, "unknown", "", err.Error(), time.Now().UTC(), "")
		return
	}
	endpoint := strings.TrimRight(agent.BaseURL, "/") + "/agent/v1/jobs/" + url.PathEscape(jobID)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var lastErr error

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+agent.Token)
			res, requestErr := m.client.Do(req)
			if requestErr == nil {
				var envelope struct {
					Status string     `json:"status"`
					Error  string     `json:"error"`
					Data   *ActionJob `json:"data"`
				}
				decodeErr := json.NewDecoder(res.Body).Decode(&envelope)
				res.Body.Close()
				if decodeErr == nil && res.StatusCode >= 200 && res.StatusCode < 300 && envelope.Data != nil {
					job := envelope.Data
					if job.Status != "running" {
						_ = m.store.CompleteScheduleRun(context.Background(), job.ID, job.Status, job.Output, job.Error, job.EndedAt, job.Duration)
						return
					}
					lastErr = nil
				} else if decodeErr != nil {
					lastErr = decodeErr
				} else if envelope.Error != "" {
					lastErr = errors.New(envelope.Error)
				} else {
					lastErr = fmt.Errorf("agent job status returned %s", res.Status)
				}
			} else {
				lastErr = requestErr
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			message := "timed out waiting for agent job status"
			if lastErr != nil {
				message += ": " + lastErr.Error()
			}
			_ = m.store.CompleteScheduleRun(context.Background(), jobID, "unknown", "", message, time.Now().UTC(), "")
			return
		case <-ticker.C:
		}
	}
}

// NextScheduleRunExported is the exported version for use by the storage
// layer when computing the initial next_run_at at save time.
func NextScheduleRunExported(schedule UpdateSchedule, after time.Time) time.Time {
	return nextScheduleRun(schedule, after)
}

func nextScheduleRun(schedule UpdateSchedule, after time.Time) time.Time {
	loc := schedulerLocation()
	switch schedule.Cadence {
	case "daily":
		return nextDaily(schedule.TimeOfDay, after, loc)
	case "weekly":
		return nextWeekly(schedule.DayOfWeek, schedule.TimeOfDay, after, loc)
	case "monthly":
		return nextMonthly(schedule.DayOfMonth, schedule.TimeOfDay, after, loc)
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

func nextDaily(timeOfDay string, after time.Time, loc *time.Location) time.Time {
	h, m := parseTimeOfDay(timeOfDay)
	a := after.In(loc)
	candidate := time.Date(a.Year(), a.Month(), a.Day(), h, m, 0, 0, loc)
	if !candidate.After(after) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate.UTC()
}

func nextWeekly(dayOfWeek int, timeOfDay string, after time.Time, loc *time.Location) time.Time {
	h, m := parseTimeOfDay(timeOfDay)
	a := after.In(loc)
	candidate := time.Date(a.Year(), a.Month(), a.Day(), h, m, 0, 0, loc)
	target := time.Weekday(dayOfWeek)
	daysAhead := int(target - candidate.Weekday())
	if daysAhead < 0 {
		daysAhead += 7
	}
	candidate = candidate.AddDate(0, 0, daysAhead)
	if !candidate.After(after) {
		candidate = candidate.AddDate(0, 0, 7)
	}
	return candidate.UTC()
}

func nextMonthly(dayOfMonth int, timeOfDay string, after time.Time, loc *time.Location) time.Time {
	h, m := parseTimeOfDay(timeOfDay)
	if dayOfMonth < 1 {
		dayOfMonth = 1
	}
	a := after.In(loc)
	candidate := clampMonthDay(a.Year(), a.Month(), dayOfMonth, h, m, loc)
	if !candidate.After(after) {
		nextMonth := a.Month() + 1
		nextYear := a.Year()
		if nextMonth > 12 {
			nextMonth = 1
			nextYear++
		}
		candidate = clampMonthDay(nextYear, nextMonth, dayOfMonth, h, m, loc)
	}
	return candidate.UTC()
}

func clampMonthDay(year int, month time.Month, day, hour, min int, loc *time.Location) time.Time {
	last := daysInMonth(year, month)
	if day > last {
		day = last
	}
	return time.Date(year, month, day, hour, min, 0, 0, loc)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
