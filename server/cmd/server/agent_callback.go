package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/config"
	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

func runAgentCallback(cfg *config.Config, engine *core.Engine) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runAgentCallbackLoop(ctx, cfg, engine)
}

func runAgentBoth(cfg *config.Config, engine *core.Engine) {
	go runAgentCallbackLoop(context.Background(), cfg, engine)
	runAgent(cfg, engine)
}

func runAgentCallbackLoop(ctx context.Context, cfg *config.Config, engine *core.Engine) {
	endpoint, err := agentCheckinEndpoint(cfg.AgentControllerURL)
	if err != nil {
		log.Fatalf("agent callback config: %v", err)
	}
	// SECURITY: Accept self-signed certs from the controller. Most Stack
	// Manager installs use auto-generated self-signed TLS. The agent
	// token provides authentication; TLS 1.2+ provides encryption even
	// with an untrusted cert. This is intentional — not a vulnerability.
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{ // nosemgrep: problem-based-packs.insecure-transport.go-stdlib.bypass-tls-verification.bypass-tls-verification
				InsecureSkipVerify: true,  //nolint:gosec // intentional for self-signed controllers
				MinVersion:         tls.VersionTLS13, // agent/peer traffic must use TLS 1.3
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resultsEndpoint, err := agentResultsEndpoint(cfg.AgentControllerURL)
	if err != nil {
		log.Fatalf("agent callback config: %v", err)
	}

	log.Printf("Stack Manager callback agent %q starting (root: %s, controller: %s)", cfg.AgentName, cfg.Root, cfg.AgentControllerURL)
	if err := sendAgentCheckin(ctx, client, endpoint, resultsEndpoint, cfg.AgentName, cfg.AgentToken, engine); err != nil {
		log.Printf("agent check-in failed: %v", err)
	}
	if cfg.AgentCheckinOnce {
		return
	}

	ticker := time.NewTicker(cfg.AgentCheckinInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("agent callback stopped")
			return
		case <-ticker.C:
			if err := sendAgentCheckin(ctx, client, endpoint, resultsEndpoint, cfg.AgentName, cfg.AgentToken, engine); err != nil {
				log.Printf("agent check-in failed: %v", err)
			}
		}
	}
}

func agentCheckinEndpoint(controllerURL string) (string, error) {
	return agentControllerPath(controllerURL, "/api/v1/agent-checkin/projects")
}

func agentResultsEndpoint(controllerURL string) (string, error) {
	return agentControllerPath(controllerURL, "/api/v1/agent-checkin/results")
}

func agentControllerPath(controllerURL, path string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(controllerURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid controller URL")
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", fmt.Errorf("controller URL must use http or https")
	}
	return strings.TrimRight(base.String(), "/") + path, nil
}

func sendAgentCheckin(ctx context.Context, client *http.Client, endpoint, resultsEndpoint, name, token string, engine *core.Engine) error {
	projects, err := engine.DiscoverProjects()
	if err != nil {
		return err
	}
	body, err := json.Marshal(core.AgentProjectCheckin{
		Name:     name,
		Projects: projects,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// The controller wraps its reply in the standard {status,data,...} envelope;
	// data.commands carries any queued actions for this agent to run now.
	var envelope struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Commands []core.AgentCommandDispatch `json:"commands"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || envelope.Status == "error" {
		if envelope.Error == "" {
			envelope.Error = res.Status
		}
		return errors.New(envelope.Error)
	}
	log.Printf("agent check-in sent %d projects", len(projects))
	if len(envelope.Data.Commands) > 0 {
		runAgentCommands(ctx, client, resultsEndpoint, name, token, engine, envelope.Data.Commands)
	}
	return nil
}

// runAgentCommands executes each dispatched command locally and reports the
// results back to the controller.
func runAgentCommands(ctx context.Context, client *http.Client, resultsEndpoint, name, token string, engine *core.Engine, commands []core.AgentCommandDispatch) {
	results := make([]core.AgentCommandResult, 0, len(commands))
	for _, cmd := range commands {
		success, output := executeAgentCommand(engine, cmd)
		log.Printf("agent command #%d %s %q -> success=%v", cmd.ID, cmd.Action, cmd.Project, success)
		results = append(results, core.AgentCommandResult{ID: cmd.ID, Success: success, Output: output})
	}
	if err := reportAgentResults(ctx, client, resultsEndpoint, name, token, results); err != nil {
		log.Printf("agent command result report failed: %v", err)
	}
}

func executeAgentCommand(engine *core.Engine, cmd core.AgentCommandDispatch) (bool, string) {
	project, err := engine.GetProject(cmd.Project)
	if err != nil {
		return false, "project not found: " + err.Error()
	}
	timeout := 300
	if cmd.Params != "" {
		var p struct {
			Timeout int `json:"timeout"`
		}
		if json.Unmarshal([]byte(cmd.Params), &p) == nil && p.Timeout > 0 {
			timeout = p.Timeout
		}
	}
	switch cmd.Action {
	case "up":
		r := engine.Up(project)
		return r.Success, r.Output
	case "down":
		r := engine.Down(project)
		return r.Success, r.Output
	case "pull":
		r := engine.Pull(project, timeout)
		return r.Success, r.Output
	case "restart":
		r := engine.Restart(project)
		return r.Success, r.Output
	case "update":
		ok := true
		var b strings.Builder
		for _, r := range engine.Update(project, timeout) {
			if !r.Success {
				ok = false
			}
			b.WriteString(r.Output)
			b.WriteString("\n")
		}
		return ok, b.String()
	default:
		return false, "unknown action: " + cmd.Action
	}
}

func reportAgentResults(ctx context.Context, client *http.Client, endpoint, name, token string, results []core.AgentCommandResult) error {
	body, err := json.Marshal(core.AgentCommandResults{Name: name, Results: results})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("results endpoint returned %s", res.Status)
	}
	log.Printf("agent reported %d command result(s)", len(results))
	return nil
}
