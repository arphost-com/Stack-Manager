package backup

import (
	"testing"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

func TestBackupScheduleSupportsCalendarCadences(t *testing.T) {
	for _, req := range []core.BackupScheduleRequest{
		{Name: "daily", Cadence: "daily", TimeOfDay: "03:00", IntervalMinutes: 1440},
		{Name: "weekly", Cadence: "weekly", TimeOfDay: "04:00", DayOfWeek: 6, IntervalMinutes: 1440},
		{Name: "monthly", Cadence: "monthly", TimeOfDay: "05:00", DayOfMonth: 15, IntervalMinutes: 1440},
		{Name: "interval", Cadence: "interval", IntervalMinutes: 60},
	} {
		if err := validateBackupSchedule(req); err != nil {
			t.Errorf("%s rejected: %v", req.Name, err)
		}
	}
}

func TestExistingIntervalBackupScheduleKeepsWorking(t *testing.T) {
	req := core.BackupScheduleRequest{Name: "legacy", IntervalMinutes: 1440}
	normalizeBackupSchedule(&req)
	if req.Cadence != "interval" {
		t.Fatalf("legacy cadence = %q", req.Cadence)
	}
	if err := validateBackupSchedule(req); err != nil {
		t.Fatal(err)
	}
	after := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	next := nextBackupRun(core.BackupSchedule{Cadence: req.Cadence, IntervalMinutes: req.IntervalMinutes}, after)
	if !next.Equal(after.Add(24 * time.Hour)) {
		t.Fatalf("next = %v", next)
	}
}
