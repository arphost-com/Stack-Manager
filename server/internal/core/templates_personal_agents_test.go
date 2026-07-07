package core

import "testing"

func TestPersonalAgentStackTemplatesBestNine(t *testing.T) {
	templates := personalAgentStackTemplates()
	wantIDs := []string{
		"openclaw",
		"zeroclaw",
		"nanoclaw",
		"nanobot",
		"hermes-agent",
		"qwenpaw",
		"openjarvis",
		"moltworker",
		"letta-agent",
	}

	if len(templates) != len(wantIDs) {
		t.Fatalf("personal agent template count = %d, want %d", len(templates), len(wantIDs))
	}

	seen := make(map[string]StackTemplate, len(templates))
	for _, template := range templates {
		seen[template.ID] = template
		if template.Category != "ai" {
			t.Fatalf("template %s category = %q, want ai", template.ID, template.Category)
		}
		if template.Subcategory != "personal-agents" {
			t.Fatalf("template %s subcategory = %q, want personal-agents", template.ID, template.Subcategory)
		}
		if template.ComposeContent == "" {
			t.Fatalf("template %s has empty compose content", template.ID)
		}
	}

	for _, id := range wantIDs {
		if _, ok := seen[id]; !ok {
			t.Fatalf("missing personal agent template %s", id)
		}
	}
}
