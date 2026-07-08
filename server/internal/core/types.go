package core

import "time"

// Project represents a discovered Docker Compose project.
type Project struct {
	Name          string              `json:"name"`
	Dir           string              `json:"dir"`
	ComposeFile   string              `json:"compose_file"`
	Inactive      bool                `json:"inactive"`
	Running       bool                `json:"running"`
	IsGit         bool                `json:"is_git"`
	Containers    []Container         `json:"containers,omitempty"`
	HasHook       map[string]bool     `json:"has_hook,omitempty"`
	ImageSources  []ImageSource       `json:"image_sources,omitempty"`
	Documentation []ProjectDoc        `json:"documentation,omitempty"`
	UpdatePolicy  ProjectUpdatePolicy `json:"update_policy,omitempty"`
	UpdateStatus  ProjectUpdateStatus `json:"update_status,omitempty"`
}

// Container represents a running Docker container.
type Container struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	State string `json:"state"`
	Ports string `json:"ports,omitempty"`
}

// ImageSource describes how a compose service gets its image.
type ImageSource struct {
	Service      string `json:"service"`
	Image        string `json:"image,omitempty"`
	Build        bool   `json:"build"`
	BuildContext string `json:"build_context,omitempty"`
	SourceType   string `json:"source_type"`
	Registry     string `json:"registry,omitempty"`
	Repository   string `json:"repository,omitempty"`
	Tag          string `json:"tag,omitempty"`
	Access       string `json:"access,omitempty"`
	Message      string `json:"message,omitempty"`
}

// ProjectDoc is a documentation file discovered inside a compose project.
type ProjectDoc struct {
	Title     string    `json:"title"`
	Path      string    `json:"path"`
	FileName  string    `json:"file_name"`
	SizeBytes int64     `json:"size_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProjectDocContent is the readable contents of a project documentation file.
type ProjectDocContent struct {
	Doc     ProjectDoc `json:"doc"`
	Content string     `json:"content"`
}

// ProjectUpdatePolicy controls whether Stack Manager should attempt image updates.
type ProjectUpdatePolicy struct {
	Mode               string `json:"mode"`
	EffectivePolicy    string `json:"effective_policy"`
	SourceType         string `json:"source_type,omitempty"`
	SourceURL          string `json:"source_url,omitempty"`
	NoUpdatesReason    string `json:"no_updates_reason,omitempty"`
	Notes              string `json:"notes,omitempty"`
	AutoDetected       bool   `json:"auto_detected"`
	DetectedPolicy     string `json:"detected_policy,omitempty"`
	DetectedSourceType string `json:"detected_source_type,omitempty"`
	DetectedSourceURL  string `json:"detected_source_url,omitempty"`
	DetectedReason     string `json:"detected_reason,omitempty"`
}

// ProjectUpdateStatus is the last known registry image update result for a project.
type ProjectUpdateStatus struct {
	Checked         bool               `json:"checked"`
	Available       bool               `json:"available"`
	Count           int                `json:"count"`
	CheckedAt       *time.Time         `json:"checked_at,omitempty"`
	NextCheckAt     *time.Time         `json:"next_check_at,omitempty"`
	Error           string             `json:"error,omitempty"`
	Images          []ImageUpdateCheck `json:"images,omitempty"`
	RegistryImages  int                `json:"registry_images"`
	SkippedServices int                `json:"skipped_services"`
}

// ImageUpdateCheck records local-vs-remote digest state for one compose service image.
type ImageUpdateCheck struct {
	Project         string    `json:"project"`
	Service         string    `json:"service"`
	Image           string    `json:"image"`
	LocalDigest     string    `json:"local_digest,omitempty"`
	RemoteDigest    string    `json:"remote_digest,omitempty"`
	Status          string    `json:"status"`
	UpdateAvailable bool      `json:"update_available"`
	Error           string    `json:"error,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
}

// CreateProjectRequest creates a compose project folder under the configured root.
type CreateProjectRequest struct {
	Name           string `json:"name"`
	ComposeContent string `json:"compose_content"`
	EnvContent     string `json:"env_content,omitempty"`
	RunAsUID       string `json:"run_as_uid,omitempty"`
	RunAsGID       string `json:"run_as_gid,omitempty"`
	EnforceUser    *bool  `json:"enforce_user,omitempty"`
	Inactive       bool   `json:"inactive,omitempty"`
	Overwrite      bool   `json:"overwrite,omitempty"`
}

// DeleteProjectRequest removes a compose project directory after exact-name confirmation.
type DeleteProjectRequest struct {
	ConfirmName string `json:"confirm_name"`
	StopFirst   bool   `json:"stop_first"`
}

// RegistryLoginRequest logs Docker into a registry using password-stdin.
type RegistryLoginRequest struct {
	Registry string `json:"registry,omitempty"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// OpResult is the result of a compose operation (pull, up, down, etc.).
type OpResult struct {
	Project  string `json:"project"`
	Action   string `json:"action"`
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Duration string `json:"duration,omitempty"`
}

// ExecResult is the raw result of running a command.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// BulkRequest is used for bulk operations on multiple projects.
type BulkRequest struct {
	Projects []string `json:"projects,omitempty"`
	Exclude  []string `json:"exclude,omitempty"`
	Timeout  int      `json:"timeout,omitempty"`
}

// PruneRequest chooses the Docker prune command to run.
type PruneRequest struct {
	Mode string `json:"mode"`
}

// BulkResult collects results from a bulk operation.
type BulkResult struct {
	Results  []OpResult `json:"results"`
	Total    int        `json:"total"`
	Success  int        `json:"success"`
	Failed   int        `json:"failed"`
	Duration string     `json:"duration,omitempty"`
}

// BackupInfo describes a stored backup.
type BackupInfo struct {
	ID           string                 `json:"id"`
	Project      string                 `json:"project"`
	File         string                 `json:"file"`
	SizeBytes    int64                  `json:"size_bytes"`
	CreatedAt    time.Time              `json:"created_at"`
	Destination  *BackupTransferResult  `json:"destination,omitempty"`
	Destinations []BackupTransferResult `json:"destinations,omitempty"`
}

// BackupCreateRequest controls backup creation and optional off-host copy.
type BackupCreateRequest struct {
	DestinationID  *int64  `json:"destination_id,omitempty"`
	DestinationIDs []int64 `json:"destination_ids,omitempty"`
}

// BackupSchedule runs project backups automatically.
type BackupSchedule struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Enabled         bool       `json:"enabled"`
	Projects        []string   `json:"projects,omitempty"`
	DestinationIDs  []int64    `json:"destination_ids,omitempty"`
	IntervalMinutes int        `json:"interval_minutes"`
	NextRunAt       time.Time  `json:"next_run_at"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	LastStatus      string     `json:"last_status,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	LastBackupIDs   []string   `json:"last_backup_ids,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// BackupScheduleRequest creates or updates an automatic backup schedule.
type BackupScheduleRequest struct {
	ID              int64      `json:"id,omitempty"`
	Name            string     `json:"name"`
	Enabled         *bool      `json:"enabled,omitempty"`
	Projects        []string   `json:"projects,omitempty"`
	DestinationIDs  []int64    `json:"destination_ids,omitempty"`
	IntervalMinutes int        `json:"interval_minutes"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
}

// BackupDestination is a configured backup endpoint.
type BackupDestination struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Enabled   bool              `json:"enabled"`
	Config    map[string]string `json:"config,omitempty"`
	HasSecret bool              `json:"has_secret"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// BackupDestinationRequest creates or updates a backup endpoint.
type BackupDestinationRequest struct {
	ID      int64             `json:"id,omitempty"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Enabled *bool             `json:"enabled,omitempty"`
	Config  map[string]string `json:"config,omitempty"`
	Secrets map[string]string `json:"secrets,omitempty"`
}

// BackupTransferResult describes a backup copy/upload attempt.
type BackupTransferResult struct {
	DestinationID   int64     `json:"destination_id"`
	DestinationName string    `json:"destination_name"`
	Type            string    `json:"type"`
	Target          string    `json:"target"`
	Success         bool      `json:"success"`
	Output          string    `json:"output,omitempty"`
	Error           string    `json:"error,omitempty"`
	CompletedAt     time.Time `json:"completed_at"`
}

// ContainerMetricSnapshot is one sampled docker stats row.
type ContainerMetricSnapshot struct {
	Project          string    `json:"project"`
	Container        string    `json:"container"`
	CPUPercent       float64   `json:"cpu_percent"`
	MemoryPercent    float64   `json:"memory_percent"`
	MemoryUsageBytes int64     `json:"memory_usage_bytes"`
	MemoryLimitBytes int64     `json:"memory_limit_bytes"`
	NetRxBytes       int64     `json:"net_rx_bytes"`
	NetTxBytes       int64     `json:"net_tx_bytes"`
	BlockReadBytes   int64     `json:"block_read_bytes"`
	BlockWriteBytes  int64     `json:"block_write_bytes"`
	PIDs             int       `json:"pids"`
	SampledAt        time.Time `json:"sampled_at"`
}

// MetricHistoryPoint is an aggregated time-series point for dashboard graphing.
type MetricHistoryPoint struct {
	SampledAt        time.Time `json:"sampled_at"`
	Project          string    `json:"project,omitempty"`
	ContainerCount   int       `json:"container_count"`
	CPUPercentAvg    float64   `json:"cpu_percent_avg"`
	MemoryPercentAvg float64   `json:"memory_percent_avg"`
	MemoryUsageBytes int64     `json:"memory_usage_bytes"`
	NetRxBytes       int64     `json:"net_rx_bytes"`
	NetTxBytes       int64     `json:"net_tx_bytes"`
}

// BackupEvent records backup/restore/delete activity for historical reporting.
type BackupEvent struct {
	Project         string    `json:"project"`
	BackupID        string    `json:"backup_id"`
	EventType       string    `json:"event_type"`
	DestinationID   int64     `json:"destination_id,omitempty"`
	DestinationName string    `json:"destination_name,omitempty"`
	Target          string    `json:"target,omitempty"`
	SizeBytes       int64     `json:"size_bytes"`
	Success         bool      `json:"success"`
	Error           string    `json:"error,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// BackupActivityPoint is an aggregated backup event bucket.
type BackupActivityPoint struct {
	BucketStart  time.Time `json:"bucket_start"`
	Backups      int       `json:"backups"`
	Restores     int       `json:"restores"`
	Deletes      int       `json:"deletes"`
	Uploads      int       `json:"uploads"`
	BackupBytes  int64     `json:"backup_bytes"`
	RestoreBytes int64     `json:"restore_bytes"`
	UploadBytes  int64     `json:"upload_bytes"`
}

// MetricsSummary is the dashboard's cached observability overview.
type MetricsSummary struct {
	LastSampledAt     *time.Time            `json:"last_sampled_at,omitempty"`
	ContainerCount    int                   `json:"container_count"`
	CPUPercentAvg     float64               `json:"cpu_percent_avg"`
	MemoryPercentAvg  float64               `json:"memory_percent_avg"`
	MemoryUsageBytes  int64                 `json:"memory_usage_bytes"`
	NetRxBytes        int64                 `json:"net_rx_bytes"`
	NetTxBytes        int64                 `json:"net_tx_bytes"`
	BackupCount24h    int                   `json:"backup_count_24h"`
	RestoreCount24h   int                   `json:"restore_count_24h"`
	BackupBytes24h    int64                 `json:"backup_bytes_24h"`
	UploadBytes24h    int64                 `json:"upload_bytes_24h"`
	ProjectSnapshots  []MetricHistoryPoint  `json:"project_snapshots,omitempty"`
	BackupActivity24h []BackupActivityPoint `json:"backup_activity_24h,omitempty"`
}

// DatabaseInfo describes a database found inside a container.
type DatabaseInfo struct {
	Container string   `json:"container"`
	Engine    string   `json:"engine"`
	Host      string   `json:"host,omitempty"`
	Databases []string `json:"databases,omitempty"`
}

// SecurityFinding represents a single security issue found during a scan.
type SecurityFinding struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Project     string `json:"project,omitempty"`
	Container   string `json:"container,omitempty"`
}

// SecurityReport is the result of a security scan.
type SecurityReport struct {
	Project   string            `json:"project"`
	Findings  []SecurityFinding `json:"findings"`
	ScannedAt time.Time         `json:"scanned_at"`
	Summary   map[string]int    `json:"summary"`
}

// ComposeAgent describes a remote Docker Compose host managed by the controller.
type ComposeAgent struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	BaseURL   string     `json:"base_url"`
	Mode      string     `json:"mode"`
	Enabled   bool       `json:"enabled"`
	LastSeen  *time.Time `json:"last_seen,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Token     string     `json:"-"`
}

// ComposeAgentRequest creates or updates a remote Docker Compose agent.
type ComposeAgentRequest struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Mode    string `json:"mode,omitempty"`
	Token   string `json:"token,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// AgentProjectSnapshot stores the last outbound check-in inventory for an agent.
type AgentProjectSnapshot struct {
	AgentID    int64     `json:"agent_id"`
	Projects   []Project `json:"projects"`
	ReceivedAt time.Time `json:"received_at"`
}

// AgentProjectCheckin is sent by outbound agents that cannot accept inbound calls.
type AgentProjectCheckin struct {
	Name     string    `json:"name"`
	Projects []Project `json:"projects"`
}

// UpdateSchedule runs compose actions automatically for a local project or agent project.
type UpdateSchedule struct {
	ID              int64      `json:"id"`
	AgentID         *int64     `json:"agent_id,omitempty"`
	AgentName       string     `json:"agent_name,omitempty"`
	Project         string     `json:"project"`
	Action          string     `json:"action"`
	Enabled         bool       `json:"enabled"`
	Cadence         string     `json:"cadence"`
	TimeOfDay       string     `json:"time_of_day"`
	DayOfWeek       int        `json:"day_of_week"`
	DayOfMonth      int        `json:"day_of_month"`
	IntervalMinutes int        `json:"interval_minutes"`
	TimeoutSeconds  int        `json:"timeout_seconds"`
	NextRunAt       time.Time  `json:"next_run_at"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	LastJobID       string     `json:"last_job_id,omitempty"`
	LastStatus      string     `json:"last_status,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// UpdateScheduleRequest creates or updates an automatic compose action schedule.
type UpdateScheduleRequest struct {
	ID              int64      `json:"id,omitempty"`
	AgentID         *int64     `json:"agent_id,omitempty"`
	Project         string     `json:"project"`
	Action          string     `json:"action,omitempty"`
	Enabled         *bool      `json:"enabled,omitempty"`
	Cadence         string     `json:"cadence,omitempty"`
	TimeOfDay       string     `json:"time_of_day,omitempty"`
	DayOfWeek       int        `json:"day_of_week,omitempty"`
	DayOfMonth      int        `json:"day_of_month,omitempty"`
	IntervalMinutes int        `json:"interval_minutes"`
	TimeoutSeconds  int        `json:"timeout_seconds,omitempty"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
}
