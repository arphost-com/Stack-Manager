package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port     int
	Root     string
	APIKey   string
	HooksDir string
	LogDir   string

	// Backup skill
	BackupDir string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8192"))

	cfg := &Config{
		Port:      port,
		Root:      getEnv("ROOT", "/docker"),
		APIKey:    getEnv("API_KEY", ""),
		HooksDir:  getEnv("HOOKS_DIR", ""),
		LogDir:    getEnv("LOG_DIR", ""),
		BackupDir: getEnv("BACKUP_DIR", ""),
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API_KEY environment variable is required")
	}

	// Default hooks dir under root
	if cfg.HooksDir == "" {
		cfg.HooksDir = cfg.Root + "/.compose-manager/hooks"
	}

	// Default backup dir under root
	if cfg.BackupDir == "" {
		cfg.BackupDir = cfg.Root + "/.compose-manager/backups"
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
