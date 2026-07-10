package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

// peerClient talks to other Stack Manager controllers over their public /api/v1.
// A peer is another full controller (not an agent): base_url is its HTTPS URL
// and token is its API key, sent as X-API-Key. TLS verification is skipped
// because controllers commonly run self-signed certs, matching agent_proxy.go.
var peerClient = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{ // nosemgrep: problem-based-packs.insecure-transport.go-stdlib.bypass-tls-verification.bypass-tls-verification
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		},
	},
}

// peerAPIBase returns the peer's /api/v1 base, tolerating a base_url that
// already includes a trailing slash or the /api/v1 suffix.
func peerAPIBase(baseURL string) string {
	b := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(b, "/api/v1") {
		return b
	}
	return b + "/api/v1"
}

// fetchPeerProjects live-fetches a peer controller's project list. rawQuery is
// forwarded so include_inactive / running_only stay consistent with the local
// view. Errors are returned so the caller can skip the peer without failing the
// whole dashboard.
func fetchPeerProjects(ctx context.Context, agent *core.ComposeAgent, rawQuery string) ([]core.Project, error) {
	if strings.TrimSpace(agent.BaseURL) == "" {
		return nil, fmt.Errorf("peer %s has no base URL", agent.Name)
	}
	// Always ask the peer for its LOCAL projects only. Forwarding source=all (or
	// an empty source) would make the peer fan out to ITS peers — and with two
	// controllers registered as peers of each other that recurses forever until
	// the request times out, so both dashboards end up showing nothing. We keep
	// the display filters (include_inactive / running_only) but pin source=local.
	q, _ := url.ParseQuery(rawQuery)
	q.Set("source", "local")
	target := peerAPIBase(agent.BaseURL) + "/projects?" + q.Encode()
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	if agent.Token != "" {
		req.Header.Set("X-API-Key", agent.Token)
	}
	resp, err := peerClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer %s returned HTTP %d", agent.Name, resp.StatusCode)
	}
	// The peer wraps its data in the standard envelope: {status, data:[...]}.
	var env struct {
		Status string         `json:"status"`
		Data   []core.Project `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	// Tag every project with this peer as its source and drop the peer's own
	// nested agent/peer projects (only surface the peer's local projects to
	// avoid transitive loops and duplicate rows).
	out := make([]core.Project, 0, len(env.Data))
	for i := range env.Data {
		p := env.Data[i]
		if p.SourceHost != "" && p.SourceHost != "local" {
			continue
		}
		p.SourceHost = agent.Name
		out = append(out, p)
	}
	return out, nil
}
