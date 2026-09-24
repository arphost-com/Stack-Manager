package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestTemplateProvenance(t *testing.T, dir, templateID string, compose []byte) {
	t.Helper()
	hash := composeSHA256(compose)
	payload, err := json.Marshal(templateProvenance{TemplateID: templateID, TemplateSHA256: hash, ComposeSHA256: hash})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, templateProvenanceFile), payload, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPersistentMountsChangedRejectsBindToNamedVolume(t *testing.T) {
	current := "services:\n  uptime-kuma:\n    volumes:\n      - ./uptime-kuma:/app/data\n"
	replacement := "services:\n  uptime-kuma:\n    volumes:\n      - uptime-kuma-data:/app/data\n"
	if !persistentMountsChanged(current, replacement) {
		t.Fatal("bind mount replaced by a named volume must require manual migration")
	}
}

func TestApplyTemplateUpdateRefusesPersistentMountMigration(t *testing.T) {
	dir := t.TempDir()
	compose := "services:\n  uptime-kuma:\n    image: louislam/uptime-kuma:1\n    volumes:\n      - ./uptime-kuma:/app/data\n"
	path := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(path, []byte(compose), 0600); err != nil {
		t.Fatal(err)
	}
	writeTestTemplateProvenance(t, dir, "uptime-kuma", []byte(compose))
	_, err := (&Engine{}).ApplyTemplateUpdate(&Project{Name: "uptime-kuma", Dir: dir, ComposeFile: path})
	if err == nil || !strings.Contains(err.Error(), "mount topology") {
		t.Fatalf("expected persistent-mount guard error, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != compose {
		t.Fatalf("compose changed despite rejected template apply: %q, %v", string(got), err)
	}
}

func TestPreviewTemplateUpdateDoesNotInferProvenanceFromProjectName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yml")
	tmpl, _ := GetBuiltinStackTemplate("docmost")
	if err := os.WriteFile(path, []byte(tmpl.ComposeContent), 0600); err != nil {
		t.Fatal(err)
	}
	preview := (&Engine{}).PreviewTemplateUpdate(&Project{Name: "docmost", Dir: dir, ComposeFile: path})
	if preview.HasTemplate {
		t.Fatal("a project name matching the catalog must not authorize template replacement")
	}
}

func TestPreviewTemplateUpdateRequiresUnmodifiedManagedCompose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yml")
	tmpl, _ := GetBuiltinStackTemplate("docmost")
	if err := os.WriteFile(path, []byte(tmpl.ComposeContent), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeTemplateProvenance(dir, "docmost"); err != nil {
		t.Fatal(err)
	}
	project := &Project{Name: "renamed-docmost", Dir: dir, ComposeFile: path}
	if preview := (&Engine{}).PreviewTemplateUpdate(project); !preview.HasTemplate {
		t.Fatal("explicitly managed project should resolve its recorded template")
	}
	if err := os.WriteFile(path, []byte("services:\n  app:\n    image: docmost/docmost:custom\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if preview := (&Engine{}).PreviewTemplateUpdate(project); preview.HasTemplate {
		t.Fatal("locally modified compose must not be offered automatic template replacement")
	}
}

func TestGenericOverwriteClearsTemplateProvenance(t *testing.T) {
	root := t.TempDir()
	engine := &Engine{RootDir: root}
	tmpl, _ := GetBuiltinStackTemplate("docmost")
	compose := tmpl.ComposeContent
	project, err := engine.CreateProject(CreateProjectRequest{
		Name: "docmost-custom", TemplateID: "docmost", ComposeContent: compose,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview := engine.PreviewTemplateUpdate(project); !preview.HasTemplate {
		t.Fatal("catalog-created project should have explicit template provenance")
	}
	project, err = engine.CreateProject(CreateProjectRequest{
		Name: "docmost-custom", ComposeContent: compose, Overwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview := engine.PreviewTemplateUpdate(project); preview.HasTemplate {
		t.Fatal("generic overwrite must clear template provenance")
	}
}

func TestCustomizedCatalogComposeIsNotEligibleForReplacement(t *testing.T) {
	root := t.TempDir()
	engine := &Engine{RootDir: root}
	project, err := engine.CreateProject(CreateProjectRequest{
		Name: "custom-docmost", TemplateID: "docmost",
		ComposeContent: "services:\n  docmost:\n    image: docmost/docmost:custom\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview := engine.PreviewTemplateUpdate(project); preview.HasTemplate {
		t.Fatal("a Compose file customized in the catalog editor must not be eligible for replacement")
	}
}

func TestMergeEnvKeepingValues_Emulatorjs(t *testing.T) {
	// A project deployed from the OLD emulatorjs template being updated to the
	// NEW one: the frontend key is added, the user's management port is kept,
	// and the now-unused key is preserved (not lost).
	existing := "EMULATORJS_PORT=3000\nEMULATORJS_MGMT_PORT=3001\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n"
	template := "EMULATORJS_FRONTEND_PORT=8082\nEMULATORJS_MGMT_PORT=3000\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n"

	merged := mergeEnvKeepingValues(existing, template)
	m := parseEnvMap(merged)

	if m["EMULATORJS_FRONTEND_PORT"] != "8082" {
		t.Errorf("new template key not added: got %q", m["EMULATORJS_FRONTEND_PORT"])
	}
	if m["EMULATORJS_MGMT_PORT"] != "3001" {
		t.Errorf("existing value not preserved for EMULATORJS_MGMT_PORT: got %q, want 3001", m["EMULATORJS_MGMT_PORT"])
	}
	if m["EMULATORJS_PORT"] != "3000" {
		t.Errorf("dropped key not preserved: EMULATORJS_PORT got %q, want 3000", m["EMULATORJS_PORT"])
	}
	if m["PUID"] != "1000" || m["TZ"] != "Etc/UTC" {
		t.Errorf("common keys mangled: PUID=%q TZ=%q", m["PUID"], m["TZ"])
	}
}

func TestMergeEnvKeepingValues_PreservesSecrets(t *testing.T) {
	// A generated secret in the project's .env must survive a template update
	// even though the template ships a change-me placeholder.
	existing := "JWT_SECRET=abc123realsecret\nDB_PASSWORD=hunter2\n"
	template := "JWT_SECRET=change-me\nDB_PASSWORD=change-me\nNEW_FLAG=on\n"
	m := parseEnvMap(mergeEnvKeepingValues(existing, template))
	if m["JWT_SECRET"] != "abc123realsecret" || m["DB_PASSWORD"] != "hunter2" {
		t.Errorf("secrets clobbered by template placeholders: %v", m)
	}
	if m["NEW_FLAG"] != "on" {
		t.Errorf("new template key not added: %q", m["NEW_FLAG"])
	}
}

func TestNewEnvKeys(t *testing.T) {
	got := newEnvKeys("A=1\nB=2\n", "A=x\nB=y\nC=z\n")
	if len(got) != 1 || got[0] != "C" {
		t.Errorf("newEnvKeys = %v, want [C]", got)
	}
}
