package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOSUpdateRespondMarksCommandFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := NewOSUpdateHandler()
	handler.respond(recorder, "apt output explaining the failure", errors.New("exit status 100"))

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
	var response APIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "error" {
		t.Fatalf("envelope status = %q, want error", response.Status)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T, want map", response.Data)
	}
	if data["output"] != "apt output explaining the failure" {
		t.Fatalf("output = %v", data["output"])
	}
	if data["success"] != false {
		t.Fatalf("success = %v, want false", data["success"])
	}
}

func TestOSUpdateRespondMarksCommandSuccess(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := NewOSUpdateHandler()
	handler.respond(recorder, "upgrade complete", nil)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response APIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T, want map", response.Data)
	}
	if data["success"] != true {
		t.Fatalf("success = %v, want true", data["success"])
	}
}

func TestParseOSUpgradeStatus(t *testing.T) {
	got := parseOSUpgradeStatus("state=completed\nexit_code=0\nstarted_at=2026-09-24T16:00:00Z\nfinished_at=2026-09-24T16:05:00Z\n--- output ---\n[os-update] upgrade complete\n")
	if got.State != "completed" || !got.Success || got.ExitCode != "0" {
		t.Fatalf("unexpected status: %#v", got)
	}
	if got.Output != "[os-update] upgrade complete" {
		t.Fatalf("output = %q", got.Output)
	}
}

func TestParseOSUpgradeStatusDoesNotMarkRunningSuccessful(t *testing.T) {
	got := parseOSUpgradeStatus("state=running\nexit_code=\n--- output ---\napt-get dist-upgrade\n")
	if got.Success || got.State != "running" {
		t.Fatalf("unexpected status: %#v", got)
	}
}
