package core

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestClassifyImageUpdateCheckMissingLocalIsNotAnUpdate(t *testing.T) {
	check := ImageUpdateCheck{Status: ImageUpdateStatusUnknown, CheckedAt: time.Now().UTC()}
	got := classifyImageUpdateCheck(check, "", fmt.Errorf("%w: daemon says no such image", errLocalImageNotFound), "sha256:remote", nil)
	if got.Status != ImageUpdateStatusMissing || got.UpdateAvailable || got.Error != "" {
		t.Fatalf("missing local image must be informational, got %+v", got)
	}
	if got.RemoteDigest != "sha256:remote" {
		t.Fatalf("remote digest was not retained: %+v", got)
	}
}

func TestClassifyImageUpdateCheckPreservesInspectFailure(t *testing.T) {
	check := ImageUpdateCheck{Status: ImageUpdateStatusUnknown}
	got := classifyImageUpdateCheck(check, "", errors.New("local image inspect failed: permission denied"), "sha256:remote", nil)
	if got.Status != ImageUpdateStatusUnknown || got.UpdateAvailable {
		t.Fatalf("inspect failure must not become an available update: %+v", got)
	}
	if got.Error != "local image inspect failed: permission denied" {
		t.Fatalf("inspect failure was hidden: %+v", got)
	}
}

func TestClassifyImageUpdateCheckDigestMismatchIsAnUpdate(t *testing.T) {
	check := ImageUpdateCheck{Status: ImageUpdateStatusUnknown}
	got := classifyImageUpdateCheck(check, "repo/image@sha256:local", nil, "sha256:remote", nil)
	if got.Status != ImageUpdateStatusAvailable || !got.UpdateAvailable {
		t.Fatalf("digest mismatch must remain an available update: %+v", got)
	}
}

func TestValidJobActionAllowsCheck(t *testing.T) {
	if !ValidJobAction("check") {
		t.Fatal("check must be accepted as a schedule job action")
	}
}
