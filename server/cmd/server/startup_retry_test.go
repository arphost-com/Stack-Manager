package main

import (
	"errors"
	"testing"
	"time"
)

func TestRetryStartupWaitsForTheDatabase(t *testing.T) {
	calls, slept := 0, 0
	got, err := retryStartup("storage init", 5, time.Second, func(time.Duration) { slept++ }, func() (string, error) {
		calls++
		if calls < 3 {
			return "", errors.New("dial tcp 172.18.0.5:3306: connect: connection refused")
		}
		return "store", nil
	})
	if err != nil || got != "store" {
		t.Fatalf("expected success on the third try, got %q, %v", got, err)
	}
	if calls != 3 || slept != 2 {
		t.Fatalf("calls=%d slept=%d, want 3 and 2", calls, slept)
	}
}

func TestRetryStartupGivesUpWithTheLastError(t *testing.T) {
	calls, slept := 0, 0
	_, err := retryStartup("storage init", 3, time.Second, func(time.Duration) { slept++ }, func() (int, error) {
		calls++
		return 0, errors.New("still down")
	})
	if err == nil || err.Error() != "still down" {
		t.Fatalf("expected the last error, got %v", err)
	}
	if calls != 3 || slept != 2 {
		t.Fatalf("calls=%d slept=%d, want 3 and 2 (no sleep after the last try)", calls, slept)
	}
}
