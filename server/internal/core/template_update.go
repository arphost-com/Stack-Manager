package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const templateProvenanceFile = ".stack-manager-template.json"

type templateProvenance struct {
	TemplateID     string `json:"template_id"`
	TemplateSHA256 string `json:"template_sha256"`
	ComposeSHA256  string `json:"compose_sha256"`
}

// TemplateUpdatePreview reports whether a running project can be updated from
// its matching catalog template, and what would change.
type TemplateUpdatePreview struct {
	HasTemplate             bool     `json:"has_template"`
	TemplateID              string   `json:"template_id,omitempty"`
	TemplateName            string   `json:"template_name,omitempty"`
	ComposeChanged          bool     `json:"compose_changed"`
	PersistentMountsChanged bool     `json:"persistent_mounts_changed"`
	NewEnvKeys              []string `json:"new_env_keys,omitempty"`
	GPUApplied              bool     `json:"gpu_applied"`
}

func composeSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func writeTemplateProvenance(projectDir, templateID string) error {
	tmpl, ok := GetBuiltinStackTemplate(templateID)
	if !ok {
		return fmt.Errorf("catalog template %q is not available", templateID)
	}
	composePath := composeFileForDir(projectDir)
	if composePath == "" {
		return fmt.Errorf("cannot record template provenance without a compose file")
	}
	content, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("read compose for template provenance: %w", err)
	}
	payload, err := json.MarshalIndent(templateProvenance{
		TemplateID:     templateID,
		TemplateSHA256: composeSHA256([]byte(tmpl.ComposeContent)),
		ComposeSHA256:  composeSHA256(content),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode template provenance: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(filepath.Join(projectDir, templateProvenanceFile), payload, 0640); err != nil {
		return fmt.Errorf("write template provenance: %w", err)
	}
	return nil
}

func templateForManagedProject(project *Project) (StackTemplate, templateProvenance, error) {
	var provenance templateProvenance
	payload, err := os.ReadFile(filepath.Join(project.Dir, templateProvenanceFile))
	if err != nil {
		if os.IsNotExist(err) {
			return StackTemplate{}, provenance, fmt.Errorf("project is not explicitly managed by a catalog template")
		}
		return StackTemplate{}, provenance, fmt.Errorf("read template provenance: %w", err)
	}
	if err := json.Unmarshal(payload, &provenance); err != nil || provenance.TemplateID == "" || provenance.TemplateSHA256 == "" || provenance.ComposeSHA256 == "" {
		return StackTemplate{}, provenance, fmt.Errorf("template provenance is invalid")
	}
	if provenance.ComposeSHA256 != provenance.TemplateSHA256 {
		return StackTemplate{}, provenance, fmt.Errorf("project compose was customized before catalog deployment")
	}
	tmpl, ok := GetBuiltinStackTemplate(provenance.TemplateID)
	if !ok {
		return StackTemplate{}, provenance, fmt.Errorf("catalog template %q is no longer available", provenance.TemplateID)
	}
	current, err := os.ReadFile(project.ComposeFile)
	if err != nil {
		return StackTemplate{}, provenance, fmt.Errorf("read current compose: %w", err)
	}
	if composeSHA256(current) != provenance.ComposeSHA256 {
		return StackTemplate{}, provenance, fmt.Errorf("project compose has local changes after catalog deployment")
	}
	return tmpl, provenance, nil
}

func normalizeComposeText(s string) string {
	// Compare ignoring trailing whitespace and blank-line noise so that a mere
	// reformat doesn't read as a change.
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimRight(l, " \t\r"))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// composeServiceMounts extracts short-syntax service mounts from conventional
// Compose YAML. If either file uses a layout this small parser cannot prove
// equivalent, persistentMountsChanged fails closed and requires manual review.
func composeServiceMounts(compose string) (map[string][]string, bool, bool) {
	services := map[string][]string{}
	inServices, inVolumes, seenVolumes := false, false, false
	known := true
	currentService := ""
	for _, raw := range strings.Split(compose, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			inServices = trimmed == "services:"
			currentService, inVolumes = "", false
			continue
		}
		if !inServices {
			continue
		}
		if indent == 2 && strings.HasSuffix(trimmed, ":") {
			currentService = strings.TrimSuffix(trimmed, ":")
			services[currentService] = nil
			inVolumes = false
			continue
		}
		if currentService == "" {
			continue
		}
		if indent == 4 && trimmed == "volumes:" {
			inVolumes, seenVolumes = true, true
			continue
		}
		if indent == 4 && strings.HasPrefix(trimmed, "volumes:") {
			// Inline and long syntax need a full YAML parser. Refuse an
			// automatic rewrite rather than guessing that storage is unchanged.
			known = false
		}
		if indent <= 4 {
			inVolumes = false
		}
		if inVolumes && indent >= 6 && strings.HasPrefix(trimmed, "- ") {
			mount := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if strings.HasPrefix(mount, "type:") {
				known = false
			}
			services[currentService] = append(services[currentService], mount)
		}
	}
	return services, seenVolumes, known
}

func persistentMountsChanged(current, replacement string) bool {
	currentMounts, currentHasMounts, currentKnown := composeServiceMounts(current)
	replacementMounts, replacementHasMounts, replacementKnown := composeServiceMounts(replacement)
	if !currentKnown || !replacementKnown || currentHasMounts != replacementHasMounts || len(currentMounts) != len(replacementMounts) {
		return true
	}
	for service, mounts := range currentMounts {
		candidate, ok := replacementMounts[service]
		if !ok || len(mounts) != len(candidate) {
			return true
		}
		for i := range mounts {
			if mounts[i] != candidate[i] {
				return true
			}
		}
	}
	return false
}

// envKey returns the KEY of a "KEY=value" line, or "" for comments/blank lines.
func envKey(line string) string {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return ""
	}
	if i := strings.IndexByte(t, '='); i > 0 {
		return strings.TrimSpace(t[:i])
	}
	return ""
}

func parseEnvMap(s string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		k := envKey(line)
		if k == "" {
			continue
		}
		t := strings.TrimSpace(line)
		m[k] = t[strings.IndexByte(t, '=')+1:]
	}
	return m
}

// newEnvKeys lists keys present in the template's env but not in the project's
// current .env — the settings a template update would introduce.
func newEnvKeys(existing, template string) []string {
	have := parseEnvMap(existing)
	var keys []string
	seen := map[string]bool{}
	for _, line := range strings.Split(template, "\n") {
		k := envKey(line)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		if _, ok := have[k]; !ok {
			keys = append(keys, k)
		}
	}
	return keys
}

// mergeEnvKeepingValues rebuilds the .env from the template (so new keys and
// comments come in) but preserves every value the user already set. Existing
// keys the template dropped are appended at the end so nothing is lost.
func mergeEnvKeepingValues(existing, template string) string {
	have := parseEnvMap(existing)
	used := map[string]bool{}
	var out []string
	for _, line := range strings.Split(strings.TrimRight(template, "\n"), "\n") {
		k := envKey(line)
		if k != "" {
			if v, ok := have[k]; ok {
				out = append(out, k+"="+v)
				used[k] = true
				continue
			}
		}
		out = append(out, line)
	}
	// Preserve existing keys the template no longer defines.
	var extra []string
	for _, line := range strings.Split(existing, "\n") {
		k := envKey(line)
		if k != "" && !used[k] {
			extra = append(extra, line)
		}
	}
	if len(extra) > 0 {
		out = append(out, "", "# --- preserved from your previous .env ---")
		out = append(out, extra...)
	}
	return strings.Join(out, "\n") + "\n"
}

// PreviewTemplateUpdate reports whether an explicitly template-managed,
// unmodified project differs from its recorded catalog template.
func (e *Engine) PreviewTemplateUpdate(project *Project) *TemplateUpdatePreview {
	tmpl, _, err := templateForManagedProject(project)
	if err != nil {
		return &TemplateUpdatePreview{HasTemplate: false}
	}
	curCompose, _ := os.ReadFile(project.ComposeFile)
	curEnv, _ := os.ReadFile(filepath.Join(project.Dir, ".env"))
	return &TemplateUpdatePreview{
		HasTemplate:             true,
		TemplateID:              tmpl.ID,
		TemplateName:            tmpl.Name,
		ComposeChanged:          normalizeComposeText(string(curCompose)) != normalizeComposeText(tmpl.ComposeContent),
		PersistentMountsChanged: persistentMountsChanged(string(curCompose), tmpl.ComposeContent),
		NewEnvKeys:              newEnvKeys(string(curEnv), tmpl.EnvContent),
		GPUApplied:              composeHasGPU(string(curCompose)),
	}
}

// composeHasGPU is a light check so the UI can warn that re-applying the
// template overwrites a project's GPU passthrough (it lives in the compose).
func composeHasGPU(compose string) bool {
	c := strings.ToLower(compose)
	return strings.Contains(c, "driver: nvidia") || strings.Contains(c, "[gpu]") || strings.Contains(c, "- gpu")
}

// ApplyTemplateUpdate rewrites an explicitly template-managed, unmodified
// project's compose.yml and migrates its .env. The old files are backed up
// (.bak-<timestamp>) first. The caller recreates the stack after.
func (e *Engine) ApplyTemplateUpdate(project *Project) (*TemplateUpdatePreview, error) {
	tmpl, _, err := templateForManagedProject(project)
	if err != nil {
		return nil, fmt.Errorf("catalog template update unavailable: %w", err)
	}
	ts := time.Now().UTC().Format("20060102-150405")

	if project.ComposeFile == "" {
		return nil, fmt.Errorf("project has no compose file to update")
	}
	curCompose, _ := os.ReadFile(project.ComposeFile)
	if persistentMountsChanged(string(curCompose), tmpl.ComposeContent) {
		return nil, fmt.Errorf("refusing catalog template apply: it changes service mount topology; review and migrate compose.yml manually")
	}
	if len(curCompose) > 0 {
		if err := os.WriteFile(project.ComposeFile+".bak-"+ts, curCompose, 0640); err != nil {
			return nil, fmt.Errorf("back up compose: %w", err)
		}
	}
	if err := os.WriteFile(project.ComposeFile, []byte(tmpl.ComposeContent), 0640); err != nil {
		return nil, fmt.Errorf("write compose: %w", err)
	}

	envPath := filepath.Join(project.Dir, ".env")
	curEnv, _ := os.ReadFile(envPath)
	if len(curEnv) > 0 {
		if err := os.WriteFile(envPath+".bak-"+ts, curEnv, 0600); err != nil {
			return nil, fmt.Errorf("back up .env: %w", err)
		}
	}
	merged := mergeEnvKeepingValues(string(curEnv), tmpl.EnvContent)
	if err := os.WriteFile(envPath, []byte(merged), 0600); err != nil {
		return nil, fmt.Errorf("write .env: %w", err)
	}
	if err := writeTemplateProvenance(project.Dir, tmpl.ID); err != nil {
		return nil, err
	}

	return &TemplateUpdatePreview{
		HasTemplate:    true,
		TemplateID:     tmpl.ID,
		TemplateName:   tmpl.Name,
		ComposeChanged: true,
		NewEnvKeys:     newEnvKeys(string(curEnv), tmpl.EnvContent),
		GPUApplied:     composeHasGPU(string(curCompose)),
	}, nil
}
