package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/arphost-com/Stack-Manager/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

const maxDownloadBackupSize = 10 << 30

var supportedDestinationTypes = map[string]bool{
	"local": true,
	"mount": true,
	"cifs":  true,
	"nfs":   true,
	"ftp":   true,
	"sftp":  true,
	"s3":    true,
}

// Skill implements the backup/restore skill.
type Skill struct {
	engine    *core.Engine
	store     *storage.Store
	backupDir string
	cancel    context.CancelFunc
}

func New() *Skill { return &Skill{} }

func (s *Skill) Name() string { return "backup" }
func (s *Skill) Description() string {
	return "Backup and restore Docker Compose projects (configs, volumes, data)"
}
func (s *Skill) Version() string { return "1.0.0" }

func (s *Skill) Init(_ context.Context, engine *core.Engine, cfg map[string]interface{}) error {
	s.engine = engine
	if store, ok := cfg["store"].(*storage.Store); ok {
		s.store = store
	}
	if dir, ok := cfg["backup_dir"].(string); ok && dir != "" {
		s.backupDir = dir
	} else {
		s.backupDir = filepath.Join(engine.RootDir, ".stack-manager", "backups")
	}
	if err := os.MkdirAll(s.backupDir, 0755); err != nil {
		return err
	}
	if s.store != nil {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		go s.runScheduler(ctx)
	}
	return nil
}

func (s *Skill) Shutdown(_ context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}
func (s *Skill) HealthCheck(_ context.Context) error {
	if _, err := os.Stat(s.backupDir); err != nil {
		return fmt.Errorf("backup directory not accessible: %w", err)
	}
	return nil
}

func (s *Skill) RegisterRoutes(r chi.Router) {
	r.Get("/destinations", s.ListDestinations)
	r.Post("/destinations", s.SaveDestination)
	r.Delete("/destinations/{destinationId}", s.DeleteDestination)
	r.Post("/destinations/{destinationId}/test", s.TestDestination)
	r.Post("/create/{name}", s.Create)
	r.Get("/schedules", s.ListSchedules)
	r.Post("/schedules", s.SaveSchedule)
	r.Delete("/schedules/{scheduleId}", s.DeleteSchedule)
	r.Post("/schedules/{scheduleId}/run", s.RunScheduleNow)
	r.Get("/list", s.List)
	r.Get("/list/{name}", s.ListProject)
	r.Get("/download/{backupId}", s.Download)
	r.Post("/restore/{name}/{backupId}", s.Restore)
	r.Delete("/{backupId}", s.Delete)
}

// Create creates a backup of a project's directory.
func (s *Skill) Create(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req core.BackupCreateRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	project, err := s.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	backup, err := s.createProjectBackup(r.Context(), project, destinationIDs(req))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, backup)
}

func (s *Skill) createProjectBackup(ctx context.Context, project *core.Project, destinationIDs []int64) (*core.BackupInfo, error) {
	timestamp := time.Now().UTC().Format("20060102_150405")
	backupName := fmt.Sprintf("%s__%s.tar.gz", project.Name, timestamp)
	backupPath := filepath.Join(s.backupDir, backupName)

	output, err := createArchive(project.Dir, backupPath)
	if err != nil {
		return nil, fmt.Errorf("backup failed: %s - %s", err.Error(), output)
	}

	info, _ := os.Stat(backupPath)
	backup := core.BackupInfo{
		ID:        backupName,
		Project:   project.Name,
		File:      backupPath,
		SizeBytes: info.Size(),
		CreatedAt: time.Now().UTC(),
	}
	s.recordBackupEvent(ctx, core.BackupEvent{
		Project:   project.Name,
		BackupID:  backupName,
		EventType: "backup",
		SizeBytes: info.Size(),
		Success:   true,
		CreatedAt: backup.CreatedAt,
	})
	for _, destinationID := range destinationIDs {
		transfer, err := s.copyToDestination(ctx, destinationID, backupPath, backupName)
		if transfer != nil {
			backup.Destinations = append(backup.Destinations, *transfer)
			if backup.Destination == nil {
				backup.Destination = transfer
			}
			s.recordBackupEvent(ctx, core.BackupEvent{
				Project:         project.Name,
				BackupID:        backupName,
				EventType:       "upload",
				DestinationID:   transfer.DestinationID,
				DestinationName: transfer.DestinationName,
				Target:          transfer.Target,
				SizeBytes:       info.Size(),
				Success:         transfer.Success,
				Error:           transfer.Error,
				CreatedAt:       transfer.CompletedAt,
			})
		}
		if err != nil {
			continue
		}
	}

	return &backup, nil
}

// ListDestinations returns configured backup destinations without secrets.
func (s *Skill) ListDestinations(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "backup destination storage is not configured")
		return
	}
	destinations, err := s.store.ListBackupDestinations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, destinations)
}

// SaveDestination creates or updates a backup destination.
func (s *Skill) SaveDestination(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "backup destination storage is not configured")
		return
	}
	var req core.BackupDestinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateDestinationRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	destination, err := s.store.SaveBackupDestination(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, destination)
}

func (s *Skill) ListSchedules(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "backup schedule storage is not configured")
		return
	}
	schedules, err := s.store.ListBackupSchedules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, schedules)
}

func (s *Skill) SaveSchedule(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "backup schedule storage is not configured")
		return
	}
	var req core.BackupScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	normalizeBackupSchedule(&req)
	if err := validateBackupSchedule(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	schedule, err := s.store.SaveBackupSchedule(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, schedule)
}

func (s *Skill) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "backup schedule storage is not configured")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "scheduleId"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}
	if err := s.store.DeleteBackupSchedule(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "backup schedule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

func (s *Skill) RunScheduleNow(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "backup schedule storage is not configured")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "scheduleId"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}
	schedule, err := s.store.GetBackupSchedule(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "backup schedule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result, err := s.runBackupSchedule(r.Context(), *schedule)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

// DeleteDestination removes a configured backup destination.
func (s *Skill) DeleteDestination(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "backup destination storage is not configured")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "destinationId"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid destination ID")
		return
	}
	if err := s.store.DeleteBackupDestination(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "backup destination not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": id})
}

// TestDestination checks whether the server can reach/write the destination.
func (s *Skill) TestDestination(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "backup destination storage is not configured")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "destinationId"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid destination ID")
		return
	}
	result, err := s.testDestination(r.Context(), id)
	status := http.StatusOK
	if err != nil {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, result)
}

// List returns all backups.
func (s *Skill) List(w http.ResponseWriter, r *http.Request) {
	backups := s.listBackups("")
	writeJSON(w, http.StatusOK, backups)
}

// ListProject returns backups for a specific project.
func (s *Skill) ListProject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	backups := s.listBackups(name)
	writeJSON(w, http.StatusOK, backups)
}

// Restore restores a project from a backup.
func (s *Skill) Restore(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	backupID := chi.URLParam(r, "backupId")

	if !validBackupID(backupID) {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}
	if !strings.HasPrefix(backupID, name+"__") {
		writeError(w, http.StatusBadRequest, "backup does not belong to project: "+name)
		return
	}

	backupPath := filepath.Join(s.backupDir, backupID)
	if _, err := os.Stat(backupPath); err != nil {
		writeError(w, http.StatusNotFound, "backup not found: "+backupID)
		return
	}
	info, _ := os.Stat(backupPath)

	project, err := s.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Stop containers first
	if project.Running {
		s.engine.Down(project)
	}

	output, err := extractArchive(project.Dir, backupPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			fmt.Sprintf("restore failed: %s - %s", err.Error(), output))
		return
	}

	// Bring containers back up
	upResult := s.engine.Up(project)
	var sizeBytes int64
	if info != nil {
		sizeBytes = info.Size()
	}
	s.recordBackupEvent(r.Context(), core.BackupEvent{
		Project:   name,
		BackupID:  backupID,
		EventType: "restore",
		SizeBytes: sizeBytes,
		Success:   true,
		CreatedAt: time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project":  name,
		"backup":   backupID,
		"restored": true,
		"up":       upResult,
	})
}

// Download streams a local backup archive to the browser.
func (s *Skill) Download(w http.ResponseWriter, r *http.Request) {
	backupID := chi.URLParam(r, "backupId")
	if !validBackupID(backupID) {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}
	backupPath := filepath.Join(s.backupDir, backupID)
	info, err := os.Stat(backupPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup not found: "+backupID)
		return
	}
	if info.Size() > maxDownloadBackupSize {
		writeError(w, http.StatusRequestEntityTooLarge, "backup is too large to download through the web UI")
		return
	}
	file, err := os.Open(backupPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", backupID))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	http.ServeContent(w, r, backupID, info.ModTime(), file)
}

// Delete removes a backup file.
func (s *Skill) Delete(w http.ResponseWriter, r *http.Request) {
	backupID := chi.URLParam(r, "backupId")

	if !validBackupID(backupID) {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	backupPath := filepath.Join(s.backupDir, backupID)
	if _, err := os.Stat(backupPath); err != nil {
		writeError(w, http.StatusNotFound, "backup not found: "+backupID)
		return
	}
	info, _ := os.Stat(backupPath)

	if err := os.Remove(backupPath); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var sizeBytes int64
	if info != nil {
		sizeBytes = info.Size()
	}
	s.recordBackupEvent(r.Context(), core.BackupEvent{
		Project:   strings.SplitN(backupID, "__", 2)[0],
		BackupID:  backupID,
		EventType: "delete",
		SizeBytes: sizeBytes,
		Success:   true,
		CreatedAt: time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": backupID,
	})
}

func (s *Skill) recordBackupEvent(ctx context.Context, event core.BackupEvent) {
	if s.store == nil {
		return
	}
	_ = s.store.SaveBackupEvent(ctx, event)
}

func validBackupID(backupID string) bool {
	if backupID == "" || strings.Contains(backupID, "/") || strings.Contains(backupID, "\\") || strings.Contains(backupID, "..") {
		return false
	}
	return strings.HasSuffix(backupID, ".tar.gz")
}

func (s *Skill) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	s.tickSchedules(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tickSchedules(ctx)
		}
	}
}

func (s *Skill) tickSchedules(ctx context.Context) {
	if s.store == nil {
		return
	}
	schedules, err := s.store.ListDueBackupSchedules(ctx, time.Now().UTC())
	if err != nil {
		return
	}
	for _, schedule := range schedules {
		_, _ = s.runBackupSchedule(ctx, schedule)
	}
}

func (s *Skill) runBackupSchedule(ctx context.Context, schedule core.BackupSchedule) (map[string]interface{}, error) {
	projects, err := s.projectsForBackupSchedule(schedule)
	if err != nil {
		next := nextBackupRun(schedule, time.Now().UTC())
		_ = s.store.MarkBackupScheduleDispatched(ctx, schedule.ID, next, nil, "failed", err.Error())
		return nil, err
	}
	backupIDs := make([]string, 0, len(projects))
	errorsOut := make([]string, 0)
	for _, project := range projects {
		backup, err := s.createProjectBackup(ctx, project, schedule.DestinationIDs)
		if err != nil {
			errorsOut = append(errorsOut, project.Name+": "+err.Error())
			continue
		}
		backupIDs = append(backupIDs, backup.ID)
	}
	status := "completed"
	errText := ""
	if len(errorsOut) > 0 {
		status = "partial"
		errText = strings.Join(errorsOut, "\n")
		if len(backupIDs) == 0 {
			status = "failed"
		}
	}
	next := nextBackupRun(schedule, time.Now().UTC())
	if err := s.store.MarkBackupScheduleDispatched(ctx, schedule.ID, next, backupIDs, status, errText); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"schedule":   schedule.Name,
		"status":     status,
		"backup_ids": backupIDs,
		"errors":     errorsOut,
	}, nil
}

func (s *Skill) projectsForBackupSchedule(schedule core.BackupSchedule) ([]*core.Project, error) {
	if len(schedule.Projects) > 0 {
		projects := make([]*core.Project, 0, len(schedule.Projects))
		for _, name := range schedule.Projects {
			project, err := s.engine.GetProject(name)
			if err != nil {
				return nil, err
			}
			projects = append(projects, project)
		}
		return projects, nil
	}
	discovered, err := s.engine.DiscoverProjects()
	if err != nil {
		return nil, err
	}
	projects := make([]*core.Project, 0, len(discovered))
	for i := range discovered {
		if discovered[i].Inactive {
			continue
		}
		projects = append(projects, &discovered[i])
	}
	return projects, nil
}

func nextBackupRun(schedule core.BackupSchedule, after time.Time) time.Time {
	next := schedule.NextRunAt
	step := time.Duration(schedule.IntervalMinutes) * time.Minute
	if step <= 0 {
		step = 24 * time.Hour
	}
	if next.IsZero() || !next.After(after) {
		next = after.Add(step)
	}
	for !next.After(after) {
		next = next.Add(step)
	}
	return next.UTC()
}

func destinationIDs(req core.BackupCreateRequest) []int64 {
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(req.DestinationIDs)+1)
	if req.DestinationID != nil && *req.DestinationID > 0 {
		seen[*req.DestinationID] = true
		ids = append(ids, *req.DestinationID)
	}
	for _, id := range req.DestinationIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func createArchive(projectDir, backupPath string) (string, error) {
	args := []string{"-czf", backupPath, "-C", filepath.Dir(projectDir), filepath.Base(projectDir)}
	output, err := exec.Command("tar", args...).CombinedOutput()
	if err == nil {
		return string(output), nil
	}
	fallbackOutput, fallbackErr := runTarInRootHelper(args...)
	if fallbackErr == nil {
		return string(fallbackOutput), nil
	}
	return string(output) + string(fallbackOutput), fmt.Errorf("%w; root helper fallback failed: %v", err, fallbackErr)
}

func extractArchive(projectDir, backupPath string) (string, error) {
	args := []string{"-xzf", backupPath, "-C", filepath.Dir(projectDir)}
	output, err := exec.Command("tar", args...).CombinedOutput()
	if err == nil {
		return string(output), nil
	}
	fallbackOutput, fallbackErr := runTarInRootHelper(args...)
	if fallbackErr == nil {
		return string(fallbackOutput), nil
	}
	return string(output) + string(fallbackOutput), fmt.Errorf("%w; root helper fallback failed: %v", err, fallbackErr)
}

func runTarInRootHelper(tarArgs ...string) ([]byte, error) {
	container, err := os.Hostname()
	if err != nil || strings.TrimSpace(container) == "" {
		return nil, fmt.Errorf("container hostname unavailable")
	}
	helperImage := os.Getenv("BASE_IMAGE_PREFIX") + "alpine:3.22"
	args := []string{"run", "--rm", "--user", "0:0", "--volumes-from", strings.TrimSpace(container), helperImage, "tar"}
	args = append(args, tarArgs...)
	return exec.Command("docker", args...).CombinedOutput()
}

func validateDestinationRequest(req core.BackupDestinationRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("destination name is required")
	}
	destType := strings.ToLower(strings.TrimSpace(req.Type))
	if !supportedDestinationTypes[destType] {
		return fmt.Errorf("unsupported destination type %q", req.Type)
	}
	if err := validateMapValues("config", req.Config); err != nil {
		return err
	}
	if err := validateMapValues("secrets", req.Secrets); err != nil {
		return err
	}
	if req.Config == nil {
		req.Config = map[string]string{}
	}
	switch destType {
	case "local", "mount", "cifs", "nfs":
		if strings.TrimSpace(req.Config["path"]) == "" {
			return errors.New("path is required for local, mount, CIFS, and NFS destinations")
		}
	case "ftp", "sftp":
		if strings.TrimSpace(req.Config["host"]) == "" {
			return errors.New("host is required for FTP and SFTP destinations")
		}
		if strings.TrimSpace(req.Config["username"]) == "" {
			return errors.New("username is required for FTP and SFTP destinations")
		}
	case "s3":
		if strings.TrimSpace(req.Config["bucket"]) == "" {
			return errors.New("bucket is required for S3 destinations")
		}
	}
	return nil
}

func normalizeBackupSchedule(req *core.BackupScheduleRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Projects = cleanStringList(req.Projects)
	req.DestinationIDs = cleanInt64List(req.DestinationIDs)
	if req.IntervalMinutes <= 0 {
		req.IntervalMinutes = 1440
	}
	if req.NextRunAt != nil {
		next := req.NextRunAt.UTC().Truncate(time.Second)
		req.NextRunAt = &next
	}
}

func validateBackupSchedule(req core.BackupScheduleRequest) error {
	if req.Name == "" {
		return errors.New("schedule name is required")
	}
	if req.IntervalMinutes < 5 {
		return errors.New("interval must be at least 5 minutes")
	}
	for _, project := range req.Projects {
		if project == "" || strings.Contains(project, "/") || strings.Contains(project, "\\") || strings.Contains(project, "..") {
			return fmt.Errorf("invalid project name %q", project)
		}
	}
	for _, id := range req.DestinationIDs {
		if id < 1 {
			return errors.New("destination ids must be positive")
		}
	}
	return nil
}

func cleanStringList(values []string) []string {
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func cleanInt64List(values []int64) []int64 {
	seen := map[int64]bool{}
	cleaned := make([]int64, 0, len(values))
	for _, value := range values {
		if value < 1 || seen[value] {
			continue
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func validateMapValues(label string, values map[string]string) error {
	for key, value := range values {
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s contains invalid newline characters", label)
		}
	}
	return nil
}

func (s *Skill) testDestination(ctx context.Context, destinationID int64) (*core.BackupTransferResult, error) {
	destination, secrets, err := s.store.GetBackupDestination(ctx, destinationID)
	if err != nil {
		return nil, err
	}
	testName := ".stack-manager-test-" + time.Now().UTC().Format("20060102150405")
	tempFile, err := os.CreateTemp("", testName)
	if err != nil {
		return nil, err
	}
	_, _ = tempFile.WriteString("stack-manager backup destination test\n")
	_ = tempFile.Close()
	defer os.Remove(tempFile.Name())

	result, err := s.transferFile(ctx, destination, secrets, tempFile.Name(), filepath.Base(tempFile.Name()))
	return result, err
}

func (s *Skill) copyToDestination(ctx context.Context, destinationID int64, backupPath, backupName string) (*core.BackupTransferResult, error) {
	destination, secrets, err := s.store.GetBackupDestination(ctx, destinationID)
	if err != nil {
		return &core.BackupTransferResult{
			DestinationID: destinationID,
			Success:       false,
			Error:         err.Error(),
			CompletedAt:   time.Now().UTC(),
		}, err
	}
	if !destination.Enabled {
		err := errors.New("backup destination is disabled")
		return &core.BackupTransferResult{
			DestinationID:   destination.ID,
			DestinationName: destination.Name,
			Type:            destination.Type,
			Success:         false,
			Error:           err.Error(),
			CompletedAt:     time.Now().UTC(),
		}, err
	}
	return s.transferFile(ctx, destination, secrets, backupPath, backupName)
}

func (s *Skill) transferFile(ctx context.Context, destination *core.BackupDestination, secrets map[string]string, sourcePath, fileName string) (*core.BackupTransferResult, error) {
	result := &core.BackupTransferResult{
		DestinationID:   destination.ID,
		DestinationName: destination.Name,
		Type:            destination.Type,
		CompletedAt:     time.Now().UTC(),
	}
	destType := strings.ToLower(destination.Type)
	switch destType {
	case "local", "mount", "cifs", "nfs":
		target, output, err := copyToMountedPath(destination.Config["path"], sourcePath, fileName)
		result.Target = target
		result.Output = output
		result.Success = err == nil
		if err != nil {
			result.Error = err.Error()
		}
		return result, err
	case "ftp", "sftp", "s3":
		target, output, err := rcloneCopy(ctx, destination, secrets, sourcePath, fileName)
		result.Target = target
		result.Output = output
		result.Success = err == nil
		if err != nil {
			result.Error = err.Error()
		}
		return result, err
	default:
		err := fmt.Errorf("unsupported destination type %q", destination.Type)
		result.Error = err.Error()
		return result, err
	}
}

func copyToMountedPath(rootPath, sourcePath, fileName string) (string, string, error) {
	if rootPath == "" {
		return "", "", errors.New("destination path is required")
	}
	if !filepath.IsAbs(rootPath) {
		return "", "", errors.New("destination path must be absolute inside the server container")
	}
	if err := os.MkdirAll(rootPath, 0750); err != nil {
		return rootPath, "", err
	}
	target := filepath.Join(rootPath, fileName)
	in, err := os.Open(sourcePath)
	if err != nil {
		return target, "", err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return target, "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return target, "", err
	}
	if err := out.Close(); err != nil {
		return target, "", err
	}
	return target, "copied to mounted path", nil
}

func rcloneCopy(ctx context.Context, destination *core.BackupDestination, secrets map[string]string, sourcePath, fileName string) (string, string, error) {
	if _, err := exec.LookPath("rclone"); err != nil {
		return "", "", errors.New("rclone is not installed in the server container")
	}
	configPath, err := writeRcloneConfig(destination, secrets)
	if err != nil {
		return "", "", err
	}
	defer os.Remove(configPath)

	target := rcloneTarget(destination, fileName)
	if target == "" {
		return "", "", errors.New("destination target could not be built")
	}
	cmd := exec.CommandContext(ctx, "rclone", "--config", configPath, "copyto", sourcePath, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return target, string(output), fmt.Errorf("%s: %s", err.Error(), strings.TrimSpace(string(output)))
	}
	return target, string(output), nil
}

func writeRcloneConfig(destination *core.BackupDestination, secrets map[string]string) (string, error) {
	var body strings.Builder
	body.WriteString("[cmbackup]\n")
	switch strings.ToLower(destination.Type) {
	case "ftp":
		body.WriteString("type = ftp\n")
		body.WriteString("host = " + destination.Config["host"] + "\n")
		body.WriteString("user = " + destination.Config["username"] + "\n")
		if password := secrets["password"]; password != "" {
			obscured, err := rcloneObscure(password)
			if err != nil {
				return "", err
			}
			body.WriteString("pass = " + obscured + "\n")
		}
		if port := destination.Config["port"]; port != "" {
			body.WriteString("port = " + port + "\n")
		}
	case "sftp":
		body.WriteString("type = sftp\n")
		body.WriteString("host = " + destination.Config["host"] + "\n")
		body.WriteString("user = " + destination.Config["username"] + "\n")
		if password := secrets["password"]; password != "" {
			obscured, err := rcloneObscure(password)
			if err != nil {
				return "", err
			}
			body.WriteString("pass = " + obscured + "\n")
		}
		if keyFile := destination.Config["key_file"]; keyFile != "" {
			body.WriteString("key_file = " + keyFile + "\n")
		}
		if port := destination.Config["port"]; port != "" {
			body.WriteString("port = " + port + "\n")
		}
	case "s3":
		body.WriteString("type = s3\n")
		body.WriteString("provider = " + defaultString(destination.Config["provider"], "Other") + "\n")
		if accessKey := secrets["access_key_id"]; accessKey != "" {
			body.WriteString("access_key_id = " + accessKey + "\n")
		}
		if secretKey := secrets["secret_access_key"]; secretKey != "" {
			body.WriteString("secret_access_key = " + secretKey + "\n")
		}
		if endpoint := destination.Config["endpoint"]; endpoint != "" {
			body.WriteString("endpoint = " + endpoint + "\n")
		}
		if region := destination.Config["region"]; region != "" {
			body.WriteString("region = " + region + "\n")
		}
		if acl := destination.Config["acl"]; acl != "" {
			body.WriteString("acl = " + acl + "\n")
		}
	default:
		return "", fmt.Errorf("unsupported rclone destination type %q", destination.Type)
	}
	file, err := os.CreateTemp("", "stack-manager-rclone-*.conf")
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(body.String()); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func rcloneObscure(password string) (string, error) {
	cmd := exec.Command("rclone", "obscure", password)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func rcloneTarget(destination *core.BackupDestination, fileName string) string {
	remotePath := strings.Trim(destination.Config["remote_path"], "/")
	switch strings.ToLower(destination.Type) {
	case "ftp", "sftp":
		if remotePath == "" {
			return "cmbackup:" + fileName
		}
		return "cmbackup:" + remotePath + "/" + fileName
	case "s3":
		bucket := strings.Trim(destination.Config["bucket"], "/")
		prefix := strings.Trim(destination.Config["prefix"], "/")
		if bucket == "" {
			return ""
		}
		if prefix == "" {
			return "cmbackup:" + bucket + "/" + fileName
		}
		return "cmbackup:" + bucket + "/" + prefix + "/" + fileName
	default:
		return ""
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (s *Skill) listBackups(projectFilter string) []core.BackupInfo {
	var backups []core.BackupInfo

	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return backups
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}

		// Parse project name from filename: <project>__<timestamp>.tar.gz
		parts := strings.SplitN(entry.Name(), "__", 2)
		if len(parts) != 2 {
			continue
		}
		projectName := parts[0]

		if projectFilter != "" && projectName != projectFilter {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, core.BackupInfo{
			ID:        entry.Name(),
			Project:   projectName,
			File:      filepath.Join(s.backupDir, entry.Name()),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	// Sort newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups
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
