package core

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type MetricsStore interface {
	SaveContainerMetricSnapshots(context.Context, []ContainerMetricSnapshot) error
	SetJSON(context.Context, string, interface{}, time.Duration)
	ResolveUpdatePolicy(Project) ProjectUpdatePolicy
}

type MetricsCollector struct {
	engine   *Engine
	store    MetricsStore
	interval time.Duration
	cacheTTL time.Duration
	stop     chan struct{}
}

func NewMetricsCollector(engine *Engine, store MetricsStore, interval, cacheTTL time.Duration) *MetricsCollector {
	if interval < 15*time.Minute {
		interval = 15 * time.Minute
	}
	if cacheTTL < interval {
		cacheTTL = interval * 2
	}
	return &MetricsCollector{
		engine:   engine,
		store:    store,
		interval: interval,
		cacheTTL: cacheTTL,
		stop:     make(chan struct{}),
	}
}

func (c *MetricsCollector) Start(ctx context.Context) {
	if c == nil || c.store == nil || c.engine == nil {
		return
	}
	ticker := time.NewTicker(c.interval)
	go func() {
		defer ticker.Stop()
		c.Collect(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stop:
				return
			case <-ticker.C:
				c.Collect(ctx)
			}
		}
	}()
}

func (c *MetricsCollector) Stop() {
	if c == nil {
		return
	}
	close(c.stop)
}

func (c *MetricsCollector) Collect(ctx context.Context) {
	projects, err := c.engine.DiscoverProjects()
	if err != nil {
		return
	}
	for i := range projects {
		projects[i].UpdatePolicy = c.store.ResolveUpdatePolicy(projects[i])
		projects[i].ImageSources = c.engine.CheckImageSources(&projects[i])
		c.store.SetJSON(ctx, "project_images:"+projects[i].Name, projects[i].ImageSources, c.cacheTTL)
	}
	c.store.SetJSON(ctx, "projects:list", projects, c.cacheTTL)

	snapshots := collectContainerStats(projects)
	if len(snapshots) > 0 {
		_ = c.store.SaveContainerMetricSnapshots(ctx, snapshots)
	}
}

func collectContainerStats(projects []Project) []ContainerMetricSnapshot {
	containerProject := map[string]string{}
	var names []string
	for _, project := range projects {
		for _, container := range project.Containers {
			if container.Name == "" {
				continue
			}
			containerProject[container.Name] = project.Name
			names = append(names, container.Name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	args := append([]string{"stats", "--no-stream", "--format", `{"container":"{{.Name}}","cpu":"{{.CPUPerc}}","memory":"{{.MemUsage}}","mem_percent":"{{.MemPerc}}","net_io":"{{.NetIO}}","block_io":"{{.BlockIO}}","pids":"{{.PIDs}}"}`}, names...)
	result, _ := DockerExec(args...)
	if result == nil {
		return nil
	}
	sampledAt := time.Now().UTC()
	snapshots := make([]ContainerMetricSnapshot, 0, len(names))
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row struct {
			Container  string `json:"container"`
			CPU        string `json:"cpu"`
			Memory     string `json:"memory"`
			MemPercent string `json:"mem_percent"`
			NetIO      string `json:"net_io"`
			BlockIO    string `json:"block_io"`
			PIDs       string `json:"pids"`
		}
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		memUsed, memLimit := parsePairBytes(row.Memory)
		netRx, netTx := parsePairBytes(row.NetIO)
		blockRead, blockWrite := parsePairBytes(row.BlockIO)
		pids, _ := strconv.Atoi(strings.TrimSpace(row.PIDs))
		snapshots = append(snapshots, ContainerMetricSnapshot{
			Project:          containerProject[row.Container],
			Container:        row.Container,
			CPUPercent:       parsePercent(row.CPU),
			MemoryPercent:    parsePercent(row.MemPercent),
			MemoryUsageBytes: memUsed,
			MemoryLimitBytes: memLimit,
			NetRxBytes:       netRx,
			NetTxBytes:       netTx,
			BlockReadBytes:   blockRead,
			BlockWriteBytes:  blockWrite,
			PIDs:             pids,
			SampledAt:        sampledAt,
		})
	}
	return snapshots
}

func parsePercent(value string) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func parsePairBytes(value string) (int64, int64) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return parseBytes(parts[0]), parseBytes(parts[1])
}

func parseBytes(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	fields := strings.Fields(value)
	if len(fields) == 2 {
		value = fields[0] + fields[1]
	}
	var numberPart strings.Builder
	var unitPart strings.Builder
	for _, r := range value {
		if (r >= '0' && r <= '9') || r == '.' {
			numberPart.WriteRune(r)
		} else if r != ' ' {
			unitPart.WriteRune(r)
		}
	}
	number, _ := strconv.ParseFloat(numberPart.String(), 64)
	switch strings.ToLower(unitPart.String()) {
	case "b", "byte", "bytes", "":
		return int64(number)
	case "kb":
		return int64(number * 1000)
	case "mb":
		return int64(number * 1000 * 1000)
	case "gb":
		return int64(number * 1000 * 1000 * 1000)
	case "tb":
		return int64(number * 1000 * 1000 * 1000 * 1000)
	case "kib":
		return int64(number * 1024)
	case "mib":
		return int64(number * 1024 * 1024)
	case "gib":
		return int64(number * 1024 * 1024 * 1024)
	case "tib":
		return int64(number * 1024 * 1024 * 1024 * 1024)
	default:
		return int64(number)
	}
}
