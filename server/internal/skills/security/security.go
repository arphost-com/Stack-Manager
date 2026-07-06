package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/go-chi/chi/v5"
)

// Skill implements the security scanning skill.
type Skill struct {
	engine *core.Engine
}

func New() *Skill { return &Skill{} }

func (s *Skill) Name() string { return "security" }
func (s *Skill) Description() string {
	return "Container security scanning, compose config auditing, and vulnerability detection"
}
func (s *Skill) Version() string { return "1.0.0" }

func (s *Skill) Init(_ context.Context, engine *core.Engine, _ map[string]interface{}) error {
	s.engine = engine
	return nil
}

func (s *Skill) Shutdown(_ context.Context) error { return nil }

func (s *Skill) HealthCheck(_ context.Context) error { return nil }

func (s *Skill) RegisterRoutes(r chi.Router) {
	r.Get("/scan/{name}", s.ScanProject)
	r.Get("/audit/{name}", s.AuditConfig)
	r.Get("/report", s.FullReport)
}

// ScanProject runs Trivy (if available) or docker scout against project images.
func (s *Skill) ScanProject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	project, err := s.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	report := core.SecurityReport{
		Project:   name,
		ScannedAt: time.Now().UTC(),
		Summary:   make(map[string]int),
	}

	// Scan each container image
	for _, c := range project.Containers {
		findings := scanImage(c.Image)
		for i := range findings {
			findings[i].Project = name
			findings[i].Container = c.Name
		}
		report.Findings = append(report.Findings, findings...)
	}

	// Config audit findings
	configFindings := auditComposeConfig(project)
	report.Findings = append(report.Findings, configFindings...)

	// Build summary
	for _, f := range report.Findings {
		report.Summary[f.Severity]++
	}

	writeJSON(w, http.StatusOK, report)
}

// AuditConfig checks the compose config for common security issues.
func (s *Skill) AuditConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	project, err := s.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	findings := auditComposeConfig(project)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project":  name,
		"findings": findings,
	})
}

// FullReport scans all running projects.
func (s *Skill) FullReport(w http.ResponseWriter, r *http.Request) {
	projects, err := s.engine.DiscoverProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var reports []core.SecurityReport
	for _, p := range projects {
		if !p.Running {
			continue
		}
		report := core.SecurityReport{
			Project:   p.Name,
			ScannedAt: time.Now().UTC(),
			Summary:   make(map[string]int),
		}

		for _, c := range p.Containers {
			findings := scanImage(c.Image)
			for i := range findings {
				findings[i].Project = p.Name
				findings[i].Container = c.Name
			}
			report.Findings = append(report.Findings, findings...)
		}

		configFindings := auditComposeConfig(&p)
		report.Findings = append(report.Findings, configFindings...)

		for _, f := range report.Findings {
			report.Summary[f.Severity]++
		}
		reports = append(reports, report)
	}

	writeJSON(w, http.StatusOK, reports)
}

// scanImage runs trivy or docker scout on an image.
func scanImage(image string) []core.SecurityFinding {
	var findings []core.SecurityFinding

	// Try trivy first
	if _, err := exec.LookPath("trivy"); err == nil {
		out, err := exec.Command("trivy", "image", "--severity", "HIGH,CRITICAL",
			"--format", "json", "--quiet", image).Output()
		if err == nil {
			findings = append(findings, parseTrivyJSON(out, image)...)
			return findings
		}
	}

	// Fallback: check for :latest tag
	if strings.HasSuffix(image, ":latest") || !strings.Contains(image, ":") {
		findings = append(findings, core.SecurityFinding{
			Severity:    "medium",
			Category:    "image-tag",
			Description: fmt.Sprintf("Image %q uses :latest or no tag — pin a specific version", image),
		})
	}

	return findings
}

// parseTrivyJSON extracts findings from trivy JSON output.
func parseTrivyJSON(data []byte, image string) []core.SecurityFinding {
	var result struct {
		Results []struct {
			Vulnerabilities []struct {
				VulnerabilityID string `json:"VulnerabilityID"`
				Severity        string `json:"Severity"`
				Title           string `json:"Title"`
				PkgName         string `json:"PkgName"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	var findings []core.SecurityFinding
	for _, r := range result.Results {
		for _, v := range r.Vulnerabilities {
			findings = append(findings, core.SecurityFinding{
				Severity:    strings.ToLower(v.Severity),
				Category:    "vulnerability",
				Description: fmt.Sprintf("[%s] %s in %s (%s)", v.VulnerabilityID, v.Title, v.PkgName, image),
			})
		}
	}
	return findings
}

// auditComposeConfig checks compose config for common security misconfigurations.
func auditComposeConfig(project *core.Project) []core.SecurityFinding {
	var findings []core.SecurityFinding

	// Read and parse compose config via docker compose config
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", project.ComposeFile, "config")
	cmd.Dir = project.Dir
	out, err := cmd.Output()
	if err != nil {
		return findings
	}

	content := string(out)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.Contains(line, "privileged: true") {
			findings = append(findings, core.SecurityFinding{
				Severity:    "high",
				Category:    "config",
				Description: "Container runs in privileged mode",
				Project:     project.Name,
			})
		}

		if strings.Contains(line, "network_mode: host") || strings.Contains(line, `network_mode: "host"`) {
			findings = append(findings, core.SecurityFinding{
				Severity:    "medium",
				Category:    "config",
				Description: "Container uses host networking",
				Project:     project.Name,
			})
		}

		if strings.Contains(line, "pid: host") || strings.Contains(line, `pid: "host"`) {
			findings = append(findings, core.SecurityFinding{
				Severity:    "high",
				Category:    "config",
				Description: "Container shares host PID namespace",
				Project:     project.Name,
			})
		}
	}

	// Check for images using :latest
	for _, c := range project.Containers {
		if strings.HasSuffix(c.Image, ":latest") || !strings.Contains(c.Image, ":") {
			findings = append(findings, core.SecurityFinding{
				Severity:    "low",
				Category:    "image-tag",
				Description: fmt.Sprintf("Container %q uses unpinned image %q", c.Name, c.Image),
				Project:     project.Name,
				Container:   c.Name,
			})
		}
	}

	return findings
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "error",
		"error":     msg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
