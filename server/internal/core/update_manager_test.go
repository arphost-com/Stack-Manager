package core

import (
	"strings"
	"testing"
	"time"
)

func TestUpdateCheckCooldownReturnsCachedResults(t *testing.T) {
	checkedAt := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	manager := &UpdateCheckManager{
		lastRunAt: checkedAt,
		lastSummary: ProjectUpdateStatus{
			Checked:   true,
			Available: true,
			Count:     1,
			Images: []ImageUpdateCheck{{
				Project: "example", Service: "web", Image: "example/web:latest", UpdateAvailable: true,
			}},
		},
	}
	got, ok := manager.cooldownResult(checkedAt.Add(time.Minute))
	if !ok {
		t.Fatal("expected a recent check to use cached results")
	}
	if !got.Cached || got.Count != 1 || got.RetryAfterSeconds != 240 {
		t.Fatalf("unexpected cached summary: %+v", got)
	}
	if got.RefreshAllowedAt == nil || !got.RefreshAllowedAt.Equal(checkedAt.Add(updateCheckCooldown)) {
		t.Fatalf("unexpected refresh time: %v", got.RefreshAllowedAt)
	}
	if !strings.Contains(got.Message, "Showing cached results") {
		t.Fatalf("missing operator message: %q", got.Message)
	}
}

func TestUpdateCheckCooldownExpires(t *testing.T) {
	checkedAt := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	manager := &UpdateCheckManager{lastRunAt: checkedAt}
	if _, ok := manager.cooldownResult(checkedAt.Add(updateCheckCooldown)); ok {
		t.Fatal("check should run again after the cooldown")
	}
}

func TestPreservePreviousUpdateResultsOnRateLimit(t *testing.T) {
	previousTime := time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC)
	failedTime := previousTime.Add(time.Hour)
	previous := ProjectUpdateStatus{
		Checked: true, Available: true, Count: 1, RegistryImages: 1, CheckedAt: &previousTime,
		Images: []ImageUpdateCheck{{
			Project: "example", Service: "web", Image: "example/web:latest",
			LocalDigest: "example/web@sha256:old", RemoteDigest: "sha256:new",
			Status: ImageUpdateStatusAvailable, UpdateAvailable: true, CheckedAt: previousTime,
		}},
	}
	current := ProjectUpdateStatus{
		Checked: true, RegistryImages: 1, CheckedAt: &failedTime,
		Error: "remote manifest unavailable: toomanyrequests: retry-after: 1m",
		Images: []ImageUpdateCheck{{
			Project: "example", Service: "web", Image: "example/web:latest",
			Status: ImageUpdateStatusUnknown, Error: "remote manifest unavailable: toomanyrequests", CheckedAt: failedTime,
		}},
	}
	got := preservePreviousUpdateResults(previous, current)
	if !got.Available || got.Count != 1 || len(got.Images) != 1 {
		t.Fatalf("cached update availability was lost: %+v", got)
	}
	if got.Images[0].RemoteDigest != "sha256:new" || got.Images[0].Status != ImageUpdateStatusAvailable {
		t.Fatalf("cached digest/status was lost: %+v", got.Images[0])
	}
	if got.Images[0].Error == "" || !got.Images[0].CheckedAt.Equal(previousTime) || !got.Cached {
		t.Fatalf("cached evidence or new failure warning was not retained truthfully: %+v", got)
	}
}

func TestPreservePreviousUpdateResultsForUncheckedImagesAfterRateLimit(t *testing.T) {
	previousTime := time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC)
	failedTime := previousTime.Add(time.Hour)
	previous := ProjectUpdateStatus{
		Checked: true, Available: true, Count: 1, RegistryImages: 2, CheckedAt: &previousTime,
		Images: []ImageUpdateCheck{
			{Project: "example", Service: "web", Image: "example/web:latest", Status: ImageUpdateStatusCurrent, CheckedAt: previousTime},
			{Project: "example", Service: "worker", Image: "example/worker:latest", Status: ImageUpdateStatusAvailable, UpdateAvailable: true, CheckedAt: previousTime},
		},
	}
	current := ProjectUpdateStatus{
		Checked: true, RegistryImages: 1, CheckedAt: &failedTime,
		Error: "remote manifest unavailable: toomanyrequests",
		Images: []ImageUpdateCheck{{
			Project: "example", Service: "web", Image: "example/web:latest",
			Status: ImageUpdateStatusUnknown, Error: "remote manifest unavailable: toomanyrequests", CheckedAt: failedTime,
		}},
	}

	got := preservePreviousUpdateResults(previous, current)
	if len(got.Images) != 2 || !got.Available || got.Count != 1 {
		t.Fatalf("unchecked cached image results were not retained: %+v", got)
	}
	if got.RegistryImages != 2 || got.Images[1].Service != "worker" || !got.Images[1].UpdateAvailable {
		t.Fatalf("unexpected retained images: %+v", got.Images)
	}
	if !got.Images[1].CheckedAt.Equal(previousTime) || !got.Cached {
		t.Fatalf("retained image should keep its last successful check time: %+v", got.Images[1])
	}
}

func TestRegistryRateLimitDetection(t *testing.T) {
	for _, message := range []string{
		"remote manifest unavailable: toomanyrequests",
		"Too Many Requests",
		"registry rate limit exceeded",
		"Retry-After: 30",
		"request failed with status code: 429",
	} {
		if !IsRegistryRateLimitError(message) {
			t.Fatalf("expected rate limit detection for %q", message)
		}
	}
	if IsRegistryRateLimitError("manifest unknown") {
		t.Fatal("ordinary manifest error must not stop the fleet check")
	}
}

func TestEligibleForScheduledUpdateCheckRequiresExplicitOptIn(t *testing.T) {
	allowed := ProjectUpdatePolicy{EffectivePolicy: UpdatePolicyAllow}
	tests := []struct {
		name       string
		project    *Project
		controller bool
		scheduled  bool
		policy     ProjectUpdatePolicy
		want       bool
	}{
		{name: "scheduled active project", project: &Project{Name: "app"}, scheduled: true, policy: allowed, want: true},
		{name: "unscheduled project", project: &Project{Name: "app"}, scheduled: false, policy: allowed},
		{name: "inactive project", project: &Project{Name: "app", Inactive: true}, scheduled: true, policy: allowed},
		{name: "controller project", project: &Project{Name: "stack-manager"}, controller: true, scheduled: true, policy: allowed},
		{name: "no updates policy", project: &Project{Name: "app"}, scheduled: true, policy: ProjectUpdatePolicy{EffectivePolicy: UpdatePolicyNoUpdates}},
		{name: "nil project", project: nil, scheduled: true, policy: allowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eligibleForScheduledUpdateCheck(tt.project, tt.controller, tt.scheduled, tt.policy); got != tt.want {
				t.Fatalf("eligibleForScheduledUpdateCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}
