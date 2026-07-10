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

	log.Printf("Stack Manager callback agent %q starting (root: %s, controller: %s)", cfg.AgentName, cfg.Root, cfg.AgentControllerURL)
	if err := sendAgentCheckin(ctx, client, endpoint, cfg.AgentName, cfg.AgentToken, engine); err != nil {
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
			if err := sendAgentCheckin(ctx, client, endpoint, cfg.AgentName, cfg.AgentToken, engine); err != nil {
				log.Printf("agent check-in failed: %v", err)
			}
		}
	}
}

func agentCheckinEndpoint(controllerURL string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(controllerURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid controller URL")
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", fmt.Errorf("controller URL must use http or https")
	}
	return strings.TrimRight(base.String(), "/") + "/api/v1/agent-checkin/projects", nil
}

func sendAgentCheckin(ctx context.Context, client *http.Client, endpoint, name, token string, engine *core.Engine) error {
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
	var envelope struct {
		Status string `json:"status"`
		Error  string `json:"error"`
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
	return nil
}
