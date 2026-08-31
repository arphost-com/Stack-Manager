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
