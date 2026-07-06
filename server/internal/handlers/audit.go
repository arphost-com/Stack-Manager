package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/storage"
)

// AuditHandler serves the /api/v1/audit endpoints. Writes come from other
// handlers/skills via Store.InsertAuditEntry — this handler is read-only.
type AuditHandler struct {
	Store *storage.Store
}

func NewAuditHandler(s *storage.Store) *AuditHandler {
	return &AuditHandler{Store: s}
}

// List returns audit rows filtered by query params:
//
//	node, actor, action, project, since, until, success, limit, offset
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := storage.AuditListParams{
		Node:    strings.TrimSpace(q.Get("node")),
		Actor:   strings.TrimSpace(q.Get("actor")),
		Action:  strings.TrimSpace(q.Get("action")),
		Project: strings.TrimSpace(q.Get("project")),
	}
	if s := q.Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			params.Since = t
		}
	}
	if s := q.Get("until"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			params.Until = t
		}
	}
	if s := q.Get("success"); s != "" {
		v := strings.EqualFold(s, "true") || s == "1"
		params.Success = &v
	}
	if s := q.Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			params.Limit = n
		}
	}
	if s := q.Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			params.Offset = n
		}
	}

	entries, err := h.Store.ListAuditEntries(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"limit":   params.Limit,
		"offset":  params.Offset,
	})
}

// Nodes returns distinct node names present in the audit log, for the
// per-node filter dropdown in the UI.
func (h *AuditHandler) Nodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.Store.DistinctAuditNodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

// Actions returns distinct action names present in the audit log.
func (h *AuditHandler) Actions(w http.ResponseWriter, r *http.Request) {
	actions, err := h.Store.DistinctAuditActions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, actions)
}
