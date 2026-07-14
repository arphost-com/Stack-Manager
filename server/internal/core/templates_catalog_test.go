package core

import "testing"

func TestBuiltinStackTemplatesFullCatalog(t *testing.T) {
	templates := BuiltinStackTemplates()
	if len(templates) != 225 {
		t.Fatalf("BuiltinStackTemplates() count = %d, want 225", len(templates))
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
