package core

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Docker's prune operates on the whole daemon. Serialize automatic cleanup
// across engines in this process; Docker also rejects concurrent prune requests.
var updateCleanupMu sync.Mutex

func updateCleanupConfig() (bool, string, error) {
	enabled := strings.TrimSpace(os.Getenv("UPDATE_CLEANUP_ENABLED"))
	if enabled == "" {
		enabled = "1"
	}
	on, err := strconv.ParseBool(enabled)
	if err != nil {
		return false, "", fmt.Errorf("UPDATE_CLEANUP_ENABLED must be a boolean")
	}
	if !on {
		return false, "", nil
	}
	age := strings.TrimSpace(os.Getenv("UPDATE_CLEANUP_MIN_AGE_HOURS"))
	if age == "" {
		age = "168"
	}
	hours, err := strconv.Atoi(age)
	if err != nil || hours < 24 || hours > 87600 {
		return false, "", fmt.Errorf("UPDATE_CLEANUP_MIN_AGE_HOURS must be an integer from 24 to 87600")
	}
	return true, strconv.Itoa(hours) + "h", nil
}

// cleanupAfterHook preserves the hook's success semantics. Verification is an
// additional gate for cleanup only: a hook can intentionally leave a stack down.
func (e *Engine) cleanupAfterHook(project *Project) string {
	enabled, _, err := updateCleanupConfig()
	if err != nil || !enabled {
		return e.cleanupAfterUpdate()
	}
	verification := e.VerifyUpdate(project)
	if !verification.Success {
		return "\n=== post-update cleanup ===\nWARNING: hook succeeded; cleanup skipped because container verification failed.\n" + verification.Output
	}
	return "\n=== verify update for cleanup ===\n" + verification.Output + e.cleanupAfterUpdate()
}

// cleanupAfterUpdate is called only after successful update verification.
// It deliberately does not use Prune's legacy modes (some include volumes).
// Cleanup failures are warnings: repeating a successful update may rerun migrations.
func (e *Engine) cleanupAfterUpdate() string {
	const heading = "\n=== post-update cleanup ===\n"
	enabled, age, err := updateCleanupConfig()
	if err != nil {
		return heading + "WARNING: cleanup skipped: " + err.Error() + ". Update succeeded.\n"
	}
	if !enabled {
		return heading + "Cleanup disabled on this host.\n"
	}
	updateCleanupMu.Lock()
	defer updateCleanupMu.Unlock()
	start := time.Now()
	var output strings.Builder
	output.WriteString(heading)
	fmt.Fprintf(&output, "Host-wide cleanup: dangling images and unused build cache older than %s; retain 1GB of build cache. No containers, networks or volumes are pruned.\n", age)
	commands := [][]string{
		{"image", "prune", "--force", "--filter", "until=" + age},
		{"builder", "prune", "--all", "--force", "--filter", "until=" + age, "--keep-storage", "1GB"},
	}
	run := e.dockerExecWithTimeout
	if run == nil {
		run = DockerExecWithTimeout
	}
	for _, args := range commands {
		fmt.Fprintf(&output, "=== docker %s ===\n", strings.Join(args, " "))
		result, runErr := run(2*time.Minute, args...)
		if result != nil {
			output.WriteString(result.Stdout)
			output.WriteString(result.Stderr)
			output.WriteByte('\n')
		}
		if runErr != nil || result == nil || result.ExitCode != 0 {
			output.WriteString("WARNING: cleanup incomplete; update succeeded. Check daemon availability and retry cleanup separately. Reclaimed space is only what Docker reported above.\n")
			log.Printf("post-update cleanup incomplete (%s, elapsed %s)", args[0], time.Since(start).Round(time.Millisecond))
			return output.String()
		}
	}
	fmt.Fprintf(&output, "Cleanup completed in %s. Reclaimed space per operation is reported by Docker above.\n", time.Since(start).Round(time.Millisecond))
	log.Printf("post-update cleanup completed in %s", time.Since(start).Round(time.Millisecond))
	return output.String()
}
