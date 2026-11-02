package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

type recordingScheduleStore struct {
	dispatched []string
	disabled   map[int64]string
}

func (s *recordingScheduleStore) ListDueSchedules(context.Context, time.Time) ([]UpdateSchedule, error) {
	return nil, nil
}
func (s *recordingScheduleStore) MarkScheduleDispatched(_ context.Context, _ int64, _ time.Time, _ string, status, errText string) error {
	s.dispatched = append(s.dispatched, status+": "+errText)
	return nil
}
func (s *recordingScheduleStore) CompleteScheduleRun(context.Context, string, string, string, string, time.Time, string) error {
	return nil
}
func (s *recordingScheduleStore) GetAgent(context.Context, int64) (*ComposeAgent, error) {
	return nil, nil
}
func (s *recordingScheduleStore) ResolveUpdatePolicy(Project) ProjectUpdatePolicy {
	return ProjectUpdatePolicy{}
}
func (s *recordingScheduleStore) EnqueueAgentCommand(context.Context, int64, AgentCommandRequest) (*AgentCommand, error) {
	return nil, nil
}
func (s *recordingScheduleStore) DisableSchedule(_ context.Context, id int64, reason string) error {
	if s.disabled == nil {
		s.disabled = map[int64]string{}
	}
	s.disabled[id] = reason
	return nil
}

func TestScheduleForDeletedProjectIsDisabled(t *testing.T) {
	store := &recordingScheduleStore{}
	manager := NewScheduleManager(NewEngine(t.TempDir(), ""), NewJobManager(nil), store)
	schedule := UpdateSchedule{ID: 7, Project: "arpvpn", Action: "update", Enabled: true, Cadence: "daily", TimeOfDay: "00:00"}
	err := manager.RunNow(context.Background(), schedule)
	if err == nil || !strings.Contains(err.Error(), "project not found: arpvpn") {
		t.Fatalf("expected project not found, got %v", err)
	}
	if len(store.dispatched) != 1 || !strings.HasPrefix(store.dispatched[0], "failed: project not found") {
		t.Fatalf("the failure should be recorded once: %v", store.dispatched)
	}
	if reason, ok := store.disabled[7]; !ok || !strings.Contains(reason, "project not found: arpvpn") {
		t.Fatalf("schedule for a missing project should be disabled, got %v", store.disabled)
	}
}
