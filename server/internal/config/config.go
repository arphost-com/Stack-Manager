package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Port          int
	Root          string
	APIKey        string
	StateDir      string
	HooksDir      string
	LogDir        string
	UsersFile     string
	JobsDir       string
	AdminUsername string
	AdminPassword string

	// Backup skill
	BackupDir string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8192"))

	cfg := &Config{
		Port:          port,
		Root:          getEnv("ROOT", "/docker"),
		APIKey:        getEnv("API_KEY", ""),
		StateDir:      getEnv("STATE_DIR", ""),
		HooksDir:      getEnv("HOOKS_DIR", ""),
		LogDir:        getEnv("LOG_DIR", ""),
		UsersFile:     getEnv("USERS_FILE", ""),
		JobsDir:       getEnv("JOBS_DIR", ""),
		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		BackupDir:     getEnv("BACKUP_DIR", ""),
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API_KEY environment variable is required")
	}

	if cfg.StateDir == "" {
		cfg.StateDir = defaultStateDir()
	}

	// Default app state paths under the configured state directory.
	if cfg.HooksDir == "" {
		cfg.HooksDir = filepath.Join(cfg.StateDir, "hooks")
	}

	if cfg.BackupDir == "" {
		cfg.BackupDir = filepath.Join(cfg.StateDir, "backups")
	}

	if cfg.UsersFile == "" {
		cfg.UsersFile = filepath.Join(cfg.StateDir, "users.json")
	}

	if cfg.JobsDir == "" {
		cfg.JobsDir = filepath.Join(cfg.StateDir, "jobs")
	}

	if cfg.AdminPassword == "" {
		cfg.AdminPassword = cfg.APIKey
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultStateDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".compose-manager")
	}
	return "/var/lib/compose-manager"
}
