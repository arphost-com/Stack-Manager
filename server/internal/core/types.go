package core

import "time"

// Project represents a discovered Docker Compose project.
type Project struct {
	Name         string              `json:"name"`
	Dir          string              `json:"dir"`
	ComposeFile  string              `json:"compose_file"`
	Inactive     bool                `json:"inactive"`
	Running      bool                `json:"running"`
	Containers   []Container         `json:"containers,omitempty"`
	HasHook      map[string]bool     `json:"has_hook,omitempty"`
	ImageSources []ImageSource       `json:"image_sources,omitempty"`
	UpdatePolicy ProjectUpdatePolicy `json:"update_policy,omitempty"`
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

// ProjectUpdatePolicy controls whether Compose Manager should attempt image updates.
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

// CreateProjectRequest creates a compose project folder under the configured root.
type CreateProjectRequest struct {
	Name           string `json:"name"`
	ComposeContent string `json:"compose_content"`
	EnvContent     string `json:"env_content,omitempty"`
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
	DestinationID *int64 `json:"destination_id,omitempty"`
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
	Token   string `json:"token,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// UpdateSchedule runs compose actions automatically for a local project or agent project.
type UpdateSchedule struct {
	ID              int64      `json:"id"`
	AgentID         *int64     `json:"agent_id,omitempty"`
	AgentName       string     `json:"agent_name,omitempty"`
	Project         string     `json:"project"`
	Action          string     `json:"action"`
	Enabled         bool       `json:"enabled"`
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
	IntervalMinutes int        `json:"interval_minutes"`
	TimeoutSeconds  int        `json:"timeout_seconds,omitempty"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
}
