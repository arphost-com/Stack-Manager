package handlers

import (
	"net/http"
	"strconv"

	"github.com/arphost-com/Compose-Manager/server/internal/core"
	"github.com/arphost-com/Compose-Manager/server/internal/storage"
)

type MetricsHandler struct {
	Store     *storage.Store
	Collector *core.MetricsCollector
}

func NewMetricsHandler(store *storage.Store, collector *core.MetricsCollector) *MetricsHandler {
	return &MetricsHandler{Store: store, Collector: collector}
}

func (h *MetricsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.Store.MetricsSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *MetricsHandler) History(w http.ResponseWriter, r *http.Request) {
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	project := r.URL.Query().Get("project")
	points, err := h.Store.MetricHistory(r.Context(), hours, project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (h *MetricsHandler) BackupActivity(w http.ResponseWriter, r *http.Request) {
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	points, err := h.Store.BackupActivity(r.Context(), hours)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (h *MetricsHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.Collector == nil {
		writeError(w, http.StatusServiceUnavailable, "metrics collector is not running")
		return
	}
	h.Collector.Collect(r.Context())
	summary, err := h.Store.MetricsSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
