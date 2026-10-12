package core

import (
	"strings"
	"testing"
)

func TestBuiltinStackTemplatesFullCatalog(t *testing.T) {
	templates := BuiltinStackTemplates()
	if len(templates) != 275 {
		t.Fatalf("BuiltinStackTemplates() count = %d, want 275", len(templates))
	}
	categories := map[string]bool{}
	for _, template := range templates {
		if template.ID == "" {
			t.Fatal("catalog contains template with empty ID")
		}
		if template.Name == "" {
			t.Fatalf("template %s has empty name", template.ID)
		}
		if template.Description == "" {
			t.Fatalf("template %s has empty description", template.ID)
		}
		if template.Category == "" {
			t.Fatalf("template %s has empty category", template.ID)
		}
		if template.ComposeContent == "" {
			t.Fatalf("template %s has empty compose content", template.ID)
		}
		categories[template.Category] = true
	}
	if len(categories) < 10 {
		t.Fatalf("catalog category count = %d, want at least 10", len(categories))
	}
	if !categories["ai"] || !categories["cms"] || !categories["database"] || !categories["media"] || !categories["proxy"] {
		t.Fatalf("catalog categories missing expected non-AI groups: %#v", categories)
	}
}

func TestOpenBrainVoiceMem0AvoidsFalcoDefaultPort(t *testing.T) {
	for _, template := range BuiltinStackTemplates() {
		if template.ID != "openbrain-voice" {
			continue
		}
		if !strings.Contains(template.ComposeContent, `"${MEM0_PORT:-18765}:8765"`) {
			t.Fatal("OpenBrain Voice mem0 should publish host port 18765 to container port 8765 by default")
		}
		if !strings.Contains(template.EnvContent, "MEM0_PORT=18765\n") {
			t.Fatal("OpenBrain Voice env defaults should match the mem0 host port mapping")
		}
		return
	}
	t.Fatal("OpenBrain Voice template not found")
}
