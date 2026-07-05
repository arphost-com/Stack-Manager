package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Mode            string
	Port            int
	Root            string
	APIKey          string
	AgentName       string
	AgentToken      string
	StateDir        string
	HooksDir        string
	LogDir          string
	AdminUsername   string
	AdminPassword   string
	DatabaseDSN     string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	CacheTTL        time.Duration
	MetricsInterval time.Duration
	WarmCacheTTL    time.Duration

	// Backup skill
	BackupDir string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8192"))

	cfg := &Config{
		Mode:          getEnv("APP_MODE", "server"),
		Port:          port,
		Root:          getEnv("ROOT", "/docker"),
		APIKey:        getEnv("API_KEY", ""),
		AgentName:     getEnv("AGENT_NAME", ""),
		AgentToken:    getEnv("AGENT_TOKEN", ""),
		StateDir:      getEnv("STATE_DIR", ""),
		HooksDir:      getEnv("HOOKS_DIR", ""),
		LogDir:        getEnv("LOG_DIR", ""),
		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		DatabaseDSN:   getEnv("DATABASE_DSN", ""),
		RedisAddr:     getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		BackupDir:     getEnv("BACKUP_DIR", ""),
	}
	cfg.RedisDB, _ = strconv.Atoi(getEnv("REDIS_DB", "0"))
	cacheTTL, _ := strconv.Atoi(getEnv("CACHE_TTL_SECONDS", "15"))
	if cacheTTL < 1 {
		cacheTTL = 15
	}
	cfg.CacheTTL = time.Duration(cacheTTL) * time.Second
	metricsMinutes, _ := strconv.Atoi(getEnv("METRICS_REFRESH_MINUTES", "15"))
	if metricsMinutes < 15 {
		metricsMinutes = 15
	}
	cfg.MetricsInterval = time.Duration(metricsMinutes) * time.Minute
	warmCacheMinutes, _ := strconv.Atoi(getEnv("WARM_CACHE_TTL_MINUTES", "30"))
	if warmCacheMinutes < metricsMinutes {
		warmCacheMinutes = metricsMinutes * 2
	}
	cfg.WarmCacheTTL = time.Duration(warmCacheMinutes) * time.Minute

	if cfg.AgentToken == "" {
		cfg.AgentToken = cfg.APIKey
	}

	if cfg.Mode == "agent" {
		if cfg.AgentToken == "" {
			return nil, fmt.Errorf("AGENT_TOKEN or API_KEY environment variable is required in agent mode")
		}
		if cfg.AgentName == "" {
			cfg.AgentName = hostnameFallback()
		}
		if cfg.StateDir == "" {
			cfg.StateDir = defaultStateDir()
		}
		if cfg.HooksDir == "" {
			cfg.HooksDir = filepath.Join(cfg.StateDir, "hooks")
		}
		return cfg, nil
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API_KEY environment variable is required")
	}

	if cfg.DatabaseDSN == "" {
		cfg.DatabaseDSN = databaseDSNFromEnv()
	}
	if cfg.DatabaseDSN == "" {
		return nil, fmt.Errorf("DATABASE_DSN or DB_HOST/DB_USER/DB_PASSWORD/DB_NAME environment variables are required")
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

func hostnameFallback() string {
	name, err := os.Hostname()
	if err == nil && name != "" {
		return name
	}
	return "compose-agent"
}

func databaseDSNFromEnv() string {
	host := getEnv("DB_HOST", "")
	user := getEnv("DB_USER", "")
	password := getEnv("DB_PASSWORD", "")
	name := getEnv("DB_NAME", "")
	port := getEnv("DB_PORT", "3306")
	if host == "" || user == "" || password == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4,utf8", user, password, host, port, name)
}
