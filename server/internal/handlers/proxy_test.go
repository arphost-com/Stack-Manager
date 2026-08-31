package handlers

import "testing"

func TestParseNPMContainerOutputDetectsExistingComposeProject(t *testing.T) {
	output := "abc123\tredis:7.4\trunning\tcache\n" +
		"def456\tdocker.io/jc21/nginx-proxy-manager:latest\texited\tlegacy-proxy\n"
	info := parseNPMContainerOutput(output)
	if info.ID != "def456" || info.State != "exited" || info.ComposeProject != "legacy-proxy" {
		t.Fatalf("unexpected NPM container info: %#v", info)
	}
}

func TestParseNPMContainerOutputIgnoresUnrelatedContainers(t *testing.T) {
	info := parseNPMContainerOutput("abc123\tnginx:stable-alpine\trunning\tweb\n")
	if info.ID != "" {
		t.Fatalf("unexpected NPM detection: %#v", info)
	}
}

func TestNPMProxyHostActionPath(t *testing.T) {
	tests := []struct {
		id      int
		enabled bool
		want    string
	}{
		{id: 12, enabled: true, want: "/api/nginx/proxy-hosts/12/enable"},
		{id: 12, enabled: false, want: "/api/nginx/proxy-hosts/12/disable"},
	}
	for _, tt := range tests {
		got, err := npmProxyHostActionPath(tt.id, tt.enabled)
		if err != nil || got != tt.want {
			t.Fatalf("npmProxyHostActionPath(%d, %v) = %q, %v; want %q", tt.id, tt.enabled, got, err, tt.want)
		}
	}
	if _, err := npmProxyHostActionPath(0, true); err == nil {
		t.Fatal("expected invalid proxy host id to fail")
	}
}
