package handlers

import (
	"strings"
	"testing"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

func TestFullPruneScheduleRequiresAcknowledgement(t *testing.T) {
	req := core.UpdateScheduleRequest{Action: "prune-system-all", Project: "anything", Cadence: "daily", TimeOfDay: "03:00"}
	normalizeSchedule(&req)
	if req.Project != "(system)" {
		t.Fatalf("project = %q", req.Project)
	}
	if err := validateSchedule(req); err == nil || !strings.Contains(err.Error(), "acknowledgement") {
		t.Fatalf("expected acknowledgement error, got %v", err)
	}
	req.ConfirmDestructive = true
	if err := validateSchedule(req); err != nil {
		t.Fatalf("acknowledged full prune rejected: %v", err)
	}
}

func TestExistingProjectScheduleStillValid(t *testing.T) {
	req := core.UpdateScheduleRequest{Action: "pull", Project: "gitlab", Cadence: "weekly", TimeOfDay: "03:00", DayOfWeek: 6}
	if err := validateSchedule(req); err != nil {
		t.Fatalf("existing pull schedule regressed: %v", err)
	}
}
