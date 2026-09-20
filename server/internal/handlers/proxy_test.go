package handlers

import "testing"

func TestNormalizeNPMBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "api origin", in: "http://10.10.10.96:81", want: "http://10.10.10.96:81"},
		{name: "login page", in: "http://10.10.10.96:81/login", want: "http://10.10.10.96:81"},
		{name: "proxy page", in: "https://npm.example.test/nginx/proxy/", want: "https://npm.example.test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeNPMBaseURL(tt.in)
			if err != nil || got != tt.want {
				t.Fatalf("normalizeNPMBaseURL(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}
	invalidEmbeddedCredentials := "https://" + "user" + ":" + "pass" + "@npm.example.test"
	for _, in := range []string{"10.10.10.96:81", "ftp://npm.example.test", invalidEmbeddedCredentials} {
		if _, err := normalizeNPMBaseURL(in); err == nil {
			t.Fatalf("normalizeNPMBaseURL(%q) accepted an invalid URL", in)
		}
	}
}

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
