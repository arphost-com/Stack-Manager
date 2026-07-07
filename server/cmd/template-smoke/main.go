package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

var requiredEnvPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*):\?[^}]*\}`)

func main() {
	mode := flag.String("mode", envDefault("STACK_TEMPLATE_SMOKE_MODE", "config"), "config or startup")
	templateID := flag.String("template", os.Getenv("STACK_TEMPLATE_ID"), "template ID to test")
	all := flag.Bool("all", envBool("STACK_TEMPLATE_ALL"), "test every stack template")
	category := flag.String("category", os.Getenv("STACK_TEMPLATE_CATEGORY"), "optional category filter")
	subcategory := flag.String("subcategory", os.Getenv("STACK_TEMPLATE_SUBCATEGORY"), "optional subcategory filter")
	workDir := flag.String("work-dir", envDefault("STACK_TEMPLATE_WORK_DIR", filepath.Join(os.TempDir(), "stack-template-smoke")), "working directory for rendered stacks")
	projectPrefix := flag.String("project-prefix", envDefault("STACK_TEMPLATE_PROJECT_PREFIX", "stack-smoke"), "docker compose project name prefix")
	settle := flag.Duration("settle", envDuration("STACK_TEMPLATE_SETTLE", 45*time.Second), "time to wait after compose up before ps check")
	keep := flag.Bool("keep", envBool("STACK_TEMPLATE_KEEP"), "keep rendered files and running containers")
	list := flag.Bool("list", envBool("STACK_TEMPLATE_LIST"), "list matching template IDs and exit")
	flag.Parse()

	if *mode != "config" && *mode != "startup" {
		log.Fatalf("unsupported mode %q: use config or startup", *mode)
	}

	templates := selectTemplates(*templateID, *all, *category, *subcategory)
	if *list {
		for _, template := range templates {
			fmt.Printf("%s\t%s\t%s\t%s\n", template.ID, template.Name, template.Category, template.Subcategory)
		}
		return
	}
	if len(templates) == 0 {
		log.Fatal("no templates selected; set STACK_TEMPLATE_ID, --template, --all, or filters")
	}

	if err := os.MkdirAll(*workDir, 0o755); err != nil {
		log.Fatal(err)
	}

	for _, template := range templates {
		if err := runTemplate(template, *mode, *workDir, *projectPrefix, *settle, *keep); err != nil {
			log.Fatalf("%s failed: %v", template.ID, err)
		}
	}
}

func selectTemplates(templateID string, all bool, category string, subcategory string) []core.StackTemplate {
	var selected []core.StackTemplate
	for _, template := range core.BuiltinStackTemplates() {
		if templateID != "" && template.ID != templateID {
			continue
		}
		if templateID == "" && !all && category == "" && subcategory == "" {
			continue
		}
		if category != "" && template.Category != category {
			continue
		}
		if subcategory != "" && template.Subcategory != subcategory {
			continue
		}
		selected = append(selected, template)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	return selected
}

func runTemplate(template core.StackTemplate, mode string, workRoot string, projectPrefix string, settle time.Duration, keep bool) error {
	dir := filepath.Join(workRoot, template.ID)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(template.ComposeContent), 0o644); err != nil {
		return err
	}
	envContent := filledEnvContent(template)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0o600); err != nil {
		return err
	}

	project := sanitizeProjectName(projectPrefix + "-" + template.ID)
	fmt.Printf("\n==> %s (%s) mode=%s project=%s\n", template.ID, template.Name, mode, project)
	if err := compose(dir, project, "config", "-q"); err != nil {
		return err
	}
	if mode == "config" {
		return nil
	}

	if !keep {
		defer func() {
			_ = compose(dir, project, "down", "-v", "--remove-orphans")
		}()
	}
	if err := compose(dir, project, "pull"); err != nil {
		return err
	}
	if err := compose(dir, project, "up", "-d", "--remove-orphans"); err != nil {
		return err
	}
	time.Sleep(settle)
	if err := compose(dir, project, "ps"); err != nil {
		return err
	}
	if err := verifyContainers(project); err != nil {
		_ = compose(dir, project, "logs", "--tail=160")
		return err
	}
	return nil
}

func compose(dir string, project string, args ...string) error {
	fullArgs := []string{"compose", "--env-file", ".env", "-f", "compose.yml", "-p", project}
	fullArgs = append(fullArgs, args...)
	cmd := exec.Command("docker", fullArgs...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func verifyContainers(project string) error {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+project, "--format", "{{.Names}}\t{{.State}}\t{{.Status}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(&out)
	var bad []string
	var total int
	for scanner.Scan() {
		total++
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		state := strings.ToLower(parts[1])
		if state != "running" {
			bad = append(bad, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if total == 0 {
		return fmt.Errorf("no containers found for compose project %s", project)
	}
	if len(bad) > 0 {
		return fmt.Errorf("containers not running: %s", strings.Join(bad, "; "))
	}
	return nil
}

func filledEnvContent(template core.StackTemplate) string {
	values := parseEnv(template.EnvContent)
	for _, match := range requiredEnvPattern.FindAllStringSubmatch(template.ComposeContent, -1) {
		if _, ok := values[match[1]]; !ok {
			values[match[1]] = ""
		}
	}

	var lines []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(template.EnvContent))
	for scanner.Scan() {
		line := scanner.Text()
		key, _, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" || strings.HasPrefix(strings.TrimSpace(key), "#") {
			lines = append(lines, line)
			continue
		}
		key = strings.TrimSpace(key)
		seen[key] = true
		lines = append(lines, key+"="+testEnvValue(key, values[key]))
	}
	var extra []string
	for key := range values {
		if !seen[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	for _, key := range extra {
		lines = append(lines, key+"="+testEnvValue(key, values[key]))
	}
	return strings.Join(lines, "\n") + "\n"
}

func parseEnv(content string) map[string]string {
	values := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}

func testEnvValue(key string, value string) string {
	if strings.TrimSpace(value) != "" && !strings.HasPrefix(value, "change-me") {
		return value
	}
	upper := strings.ToUpper(key)
	switch {
	case strings.Contains(upper, "PORT"):
		return value
	case strings.Contains(upper, "USER") || strings.Contains(upper, "USERNAME"):
		return envFallback(value, "smoke")
	case strings.Contains(upper, "EMAIL"):
		return envFallback(value, "smoke@example.invalid")
	case strings.Contains(upper, "MODEL"):
		return value
	case strings.Contains(upper, "URL"):
		return value
	default:
		return "smoke-" + randomHex(18)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func sanitizeProjectName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}
	return strings.Trim(b.String(), "-")
}

func envDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envFallback(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func envBool(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes"
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}
