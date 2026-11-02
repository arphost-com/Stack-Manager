package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuditFailureDetailsNamesFailedBulkProjects(t *testing.T) {
	body := `{"status":"error","error":"one or more project actions failed","data":{"results":[
		{"project":"pi-hole","action":"pull","success":false},
		{"project":"gitlab","action":"pull","success":true},
		{"project":"docmost","action":"pull","success":false}]}}`
	got := auditFailureDetails(http.StatusConflict, []byte(body))
	want := "one or more project actions failed; failed: pi-hole, docmost"
	if got != want {
		t.Fatalf("details = %q, want %q", got, want)
	}
}

func TestAuditFailureDetailsPlainErrorAndNonJSON(t *testing.T) {
	if got := auditFailureDetails(http.StatusBadRequest, []byte(`{"status":"error","error":"invalid action: nope"}`)); got != "invalid action: nope" {
		t.Fatalf("plain error = %q", got)
	}
	if got := auditFailureDetails(http.StatusBadGateway, []byte("<html>bad gateway</html>")); got != "HTTP 502" {
		t.Fatalf("non JSON = %q", got)
	}
	if got := auditFailureDetails(http.StatusOK, []byte(`{"error":"x"}`)); got != "" {
		t.Fatalf("success must have no details, got %q", got)
	}
}

func TestAuditRecorderOnlyBuffersErrorBodies(t *testing.T) {
	ok := &auditRecorder{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	_, _ = ok.Write([]byte(`{"status":"ok"}`))
	if ok.body.Len() != 0 {
		t.Fatal("successful responses must not be buffered")
	}
	failed := &auditRecorder{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	failed.WriteHeader(http.StatusConflict)
	_, _ = failed.Write([]byte(strings.Repeat("x", auditBodyLimit+100)))
	if failed.body.Len() != auditBodyLimit {
		t.Fatalf("error body should be capped at %d bytes, got %d", auditBodyLimit, failed.body.Len())
	}
}
