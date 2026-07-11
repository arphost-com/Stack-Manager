package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata" // embed the tz database so LoadLocation works in the container (scheduler timezone)

	cmauth "github.com/arphost-com/Stack-Manager/server/internal/auth"
	"github.com/arphost-com/Stack-Manager/server/internal/config"
	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/arphost-com/Stack-Manager/server/internal/handlers"
	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
	"github.com/arphost-com/Stack-Manager/server/internal/skills"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/backup"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/dbadmin"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/debug"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/firewall"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/frontend"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/security"
	"github.com/arphost-com/Stack-Manager/server/internal/storage"
	"github.com/arphost-com/Stack-Manager/server/internal/version"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// applyDBSettingOverrides lets DB-stored runtime settings (Settings > General,
// table app_settings) override the .env-derived config at startup, so .env is
// only the initial seed. Boot settings (ports, DB/Redis creds, cache TTL that
// configures the store) stay in .env by necessity.
func applyDBSettingOverrides(ctx context.Context, store *storage.Store, cfg *config.Config, engine *core.Engine) {
	if store == nil {
		return
	}
	if v := store.SettingStringOr(ctx, "METRICS_REFRESH_MINUTES", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			cfg.MetricsInterval = time.Duration(n) * time.Minute
		}
	}
	if v := store.SettingStringOr(ctx, "WARM_CACHE_TTL_MINUTES", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			cfg.WarmCacheTTL = time.Duration(n) * time.Minute
		}
	}
	if v := store.SettingStringOr(ctx, "CACHE_TTL_SECONDS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			store.CacheTTL = time.Duration(n) * time.Second
		}
	}
	if v := store.SettingStringOr(ctx, "DOCKER_DAEMON_DIR", ""); v != "" {
		cfg.DockerDaemonDir = v
	}
	if v := store.SettingStringOr(ctx, "EXTRA_DOCKER_ROOTS", ""); v != "" {
		roots := []string{}
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				roots = append(roots, p)
			}
		}
		cfg.ExtraRoots = roots
		engine.ExtraRoots = roots
	}
	if v := store.SettingStringOr(ctx, "HOST_URL", ""); v != "" {
		_ = os.Setenv("HOST_URL", v)
	}
	if v := store.SettingStringOr(ctx, "TZ", ""); v != "" {
		_ = os.Setenv("TZ", v)
	}
}

// loadOrSeedAPIKey returns the current API key from the DB (app_settings),
// seeding it from the .env value on first run, or generating one if .env has
// none. Storing it in the DB lets the roll endpoint change it live.
func loadOrSeedAPIKey(ctx context.Context, store *storage.Store, envKey string) string {
	if k, ok := store.GetSettingString(ctx, "api_key"); ok && strings.TrimSpace(k) != "" {
		return k
	}
	k := strings.TrimSpace(envKey)
	if k == "" {
		if gen, err := handlers.GenerateAPIKey(); err == nil {
			k = gen
			log.Printf("Stack Manager: generated a new API key (shown once): %s", k)
		}
	}
	if k != "" {
		_ = store.SetSettingString(ctx, "api_key", k)
	}
	return k
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Core engine
	engine := core.NewEngine(cfg.Root, cfg.HooksDir)
	engine.ExtraRoots = cfg.ExtraRoots
	if cfg.Mode == "agent-both" {
		runAgentBoth(cfg, engine)
		return
	}
	if cfg.Mode == "agent-callback" || cfg.Mode == "agent-cli" {
		runAgentCallback(cfg, engine)
		return
	}
	if cfg.Mode == "agent" {
		runAgent(cfg, engine)
		return
	}
	appStore, err := storage.New(context.Background(), cfg.DatabaseDSN, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.CacheTTL)
	if err != nil {
		log.Fatalf("storage init: %v", err)
	}
	defer appStore.Close()
	if err := appStore.ImportLegacyFiles(context.Background(), cfg.StateDir); err != nil {
		log.Fatalf("legacy import: %v", err)
	}
	// DB-stored runtime settings (Settings > General) override the .env-derived
	// config here, before the managers/engine consume it. .env stays the seed.
	applyDBSettingOverrides(context.Background(), appStore, cfg, engine)

	// Stamp the running build's version into the DB so the footer reflects the
	// deployed commit on EVERY rebuild path (deploy.sh, UI self-update, bare
	// compose) — not just when deploy.sh happens to PUT it. Only when a SHA was
	// baked in, so a plain `go build` (GitSHA empty) can't clobber a good value
	// with the bare base version.
	if version.GitSHA != "" {
		if err := appStore.SetSettingString(context.Background(), "app_version", version.Full()); err != nil {
			log.Printf("warning: could not stamp app_version: %v", err)
		}
	}

	jobs := core.NewJobManager(appStore)
	scheduler := core.NewScheduleManager(engine, jobs, appStore)
	metricsCollector := core.NewMetricsCollector(engine, appStore, cfg.MetricsInterval, cfg.WarmCacheTTL)
	updateChecker := core.NewUpdateCheckManager(engine, appStore)
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	scheduler.Start(schedulerCtx)
	metricsCollector.Start(schedulerCtx)
	updateChecker.Start(schedulerCtx)
	userStore, err := cmauth.NewStore(appStore, cfg.AdminUsername, cfg.AdminPassword)
	if err != nil {
		log.Fatalf("users init: %v", err)
	}
	sessionManager := cmauth.NewSessionManager(appStore, 12*time.Hour)

	// Skill registry
	registry := skills.NewRegistry()
	registry.Register(security.New())
	registry.Register(debug.New())
	registry.Register(backup.New())
	registry.Register(dbadmin.New())
	registry.Register(frontend.New())
	firewallSkill := firewall.New()
	registry.Register(firewallSkill)

	skillCfg := map[string]interface{}{
		"backup_dir": cfg.BackupDir,
		"store":      appStore,
	}

	ctx := context.Background()
	if err := registry.InitAll(ctx, engine, skillCfg); err != nil {
		log.Fatalf("skills init: %v", err)
	}

	// Handlers
	projectHandler := handlers.NewProjectHandler(engine, jobs, appStore)
	projectHandler.SetUpdateCheckManager(updateChecker)
	projectHandler.PortSyncer = firewallSkill
	agentHandler := handlers.NewAgentHandler(appStore)
	systemInfoHandler := handlers.NewSystemInfoHandler(appStore)
	gpuSetupHandler := handlers.NewGPUSetupHandler()
	osUpdateHandler := handlers.NewOSUpdateHandler()
	selfUpdateHandler := handlers.NewSelfUpdateHandler()
	systemTZHandler := handlers.NewSystemTZHandler()
	agentCheckinHandler := handlers.NewAgentCheckinHandler(appStore)
	scheduleHandler := handlers.NewScheduleHandler(appStore, scheduler)
	metricsHandler := handlers.NewMetricsHandler(appStore, metricsCollector)
	dockerSettingsHandler := handlers.NewDockerSettingsHandler(cfg.DockerDaemonDir, cfg.BaseImagePrefix)
	sslHandler := handlers.NewSSLHandler(cfg.StateDir, cfg.BaseImagePrefix)
	auditHandler := handlers.NewAuditHandler(appStore)
	watchManager := core.NewWatchManager(engine, cfg.StateDir)
	defer watchManager.Shutdown()
	watchHandler := handlers.NewWatchHandler(watchManager)
	skillHandler := handlers.NewSkillHandler(registry)
	shellHandler := handlers.NewShellHandler(engine)
	projectFileHandler := handlers.NewProjectFileHandler(engine)
	envSettingsHandler := handlers.NewEnvSettingsHandler(cfg.StateDir, appStore)
	// API key lives in the DB (seeded from .env). A thread-safe holder lets the
	// roll endpoint change it live without a restart, and the auth middleware
	// reads the current value each request.
	var apiKeyMu sync.RWMutex
	currentAPIKey := loadOrSeedAPIKey(context.Background(), appStore, cfg.APIKey)
	getAPIKey := func() string { apiKeyMu.RLock(); defer apiKeyMu.RUnlock(); return currentAPIKey }
	envSettingsHandler.SetAPIKeyUpdater(func(k string) { apiKeyMu.Lock(); currentAPIKey = k; apiKeyMu.Unlock() })
	agentProxyHandler := handlers.NewAgentProxyHandler(appStore)
	proxyHandler := handlers.NewProxyHandler(engine, appStore)
	authHandler := handlers.NewAuthHandler(userStore, sessionManager)
	authHandler.IPAllower = firewallSkill

	// Router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	// Trust X-Real-IP from the built-in nginx container unconditionally.
	// chi's RealIP only trusts loopback, but nginx connects from Docker
	// bridge IPs (172.x.x.x) which chi rejects.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
				r.RemoteAddr = rip + ":0"
			} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				if idx := strings.Index(xff, ","); idx > 0 {
					r.RemoteAddr = strings.TrimSpace(xff[:idx]) + ":0"
				} else {
					r.RemoteAddr = strings.TrimSpace(xff) + ":0"
				}
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Use(chimw.Timeout(5 * time.Minute))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Health check (public)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/totp/login", authHandler.TOTPLogin)
		r.Post("/agent-checkin/projects", agentCheckinHandler.Projects)
		r.Post("/agent-checkin/results", agentCheckinHandler.Results)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(getAPIKey, sessionManager))
			r.Use(middleware.AuditRecorder(appStore, os.Getenv("AUDIT_NODE_NAME")))

			r.Get("/auth/me", authHandler.Me)
			r.Post("/auth/logout", authHandler.Logout)
			r.Post("/auth/change-password", authHandler.ChangeOwnPassword)
			r.Post("/auth/totp/enroll", authHandler.TOTPEnroll)
			r.Post("/auth/totp/verify", authHandler.TOTPVerify)
			r.Post("/auth/totp/disable", authHandler.TOTPDisable)
			r.Delete("/users/{username}/totp", authHandler.TOTPResetUser)
			r.Get("/users", authHandler.ListUsers)
			r.Post("/users", authHandler.CreateUser)
			r.Put("/users/{username}/password", authHandler.SetPassword)
			r.Delete("/users/{username}", authHandler.DeleteUser)

			// Project endpoints
			r.Post("/projects", projectHandler.Create)
			r.Get("/projects", projectHandler.List)
			r.Get("/projects/{name}", projectHandler.Get)
			r.Delete("/projects/{name}", projectHandler.Delete)
			r.Get("/projects/{name}/docs", projectHandler.Docs)
			r.Get("/projects/{name}/docs/content", projectHandler.DocContent)
			r.Get("/projects/{name}/images", projectHandler.Images)
			r.Get("/projects/{name}/status", projectHandler.Status)
			r.Post("/projects/{name}/pull", projectHandler.Pull)
			r.Post("/projects/{name}/up", projectHandler.Up)
			r.Post("/projects/{name}/down", projectHandler.Down)

			// Up + Watch: persistent live-tail startup logs. Refresh-safe.
			r.Post("/projects/{name}/watch", watchHandler.Start)
			r.Get("/projects/{name}/watch", watchHandler.List)
			r.Get("/projects/{name}/watch/{sessionID}", watchHandler.Get)
			r.Get("/projects/{name}/watch/{sessionID}/stream", watchHandler.Stream)
			r.Delete("/projects/{name}/watch/{sessionID}", watchHandler.Stop)

			r.Get("/projects/{name}/shell/containers", shellHandler.ListContainers)
			r.Get("/projects/{name}/shell/exec", shellHandler.ExecWebSocket)

			r.Get("/projects/{name}/files", projectFileHandler.ListFiles)
			r.Get("/projects/{name}/files/content", projectFileHandler.ReadFile)
			r.Put("/projects/{name}/files/content", projectFileHandler.WriteFile)

			r.Post("/projects/{name}/update", projectHandler.Update)
			r.Post("/projects/{name}/restart", projectHandler.Restart)
			r.Post("/projects/{name}/jobs/{action}", projectHandler.StartJob)
			r.Get("/projects/{name}/update-policy", projectHandler.GetUpdatePolicy)
			r.Put("/projects/{name}/update-policy", projectHandler.SetUpdatePolicy)
			r.Put("/projects/{name}/inactive", projectHandler.SetInactive)

			// Bulk operations
			r.Post("/projects/bulk/{action}", projectHandler.BulkAction)
			r.Post("/updates/check", projectHandler.CheckUpdates)

			// System
			r.Post("/prune", projectHandler.Prune)
			r.Post("/registries/login", projectHandler.RegistryLogin)
			r.Get("/registries", projectHandler.ListRegistryLogins)
			r.Delete("/registries/{registry}", projectHandler.DeleteRegistryLogin)
			r.Get("/jobs", projectHandler.ListJobs)
			r.Get("/jobs/{jobId}", projectHandler.GetJob)
			r.Get("/stack-templates", handlers.ListStackTemplates)
			r.Get("/stack-templates/{templateID}", handlers.GetStackTemplate)
			r.Get("/agents", agentHandler.List)
			r.Post("/agents", agentHandler.Save)
			r.Get("/agents/{agentID}/projects", agentHandler.Projects)
			r.Post("/agents/{agentID}/commands", agentHandler.EnqueueCommand)
			r.Get("/agents/{agentID}/commands", agentHandler.ListCommands)
			r.Delete("/agents/{agentID}", agentHandler.Delete)
			r.Get("/schedules", scheduleHandler.List)
			r.Post("/schedules", scheduleHandler.Save)
			r.Delete("/schedules/{scheduleID}", scheduleHandler.Delete)
			r.Post("/schedules/{scheduleID}/run", scheduleHandler.RunNow)
			r.Get("/metrics/summary", metricsHandler.Summary)
			r.Get("/metrics/history", metricsHandler.History)
			r.Get("/metrics/backup-activity", metricsHandler.BackupActivity)
			r.Post("/metrics/refresh", metricsHandler.Refresh)
			r.Get("/docker/daemon", dockerSettingsHandler.GetDaemon)
			r.Put("/docker/daemon", dockerSettingsHandler.SaveDaemon)
			r.Post("/docker/restart", dockerSettingsHandler.RestartDocker)

			// Reverse proxy (NPM)
			r.Get("/proxy/status", proxyHandler.Status)
			r.Post("/proxy/deploy", proxyHandler.DeployNPM)
			r.Post("/proxy/configure", proxyHandler.Configure)
			r.Get("/proxy/hosts", proxyHandler.ListHosts)
			r.Post("/proxy/hosts", proxyHandler.CreateHost)
			r.Delete("/proxy/hosts", proxyHandler.DeleteHost)
			r.Get("/proxy/suggestions", proxyHandler.ProjectSuggestions)

			// System info
			r.Get("/system/gpu", handlers.GPUDetect)
			r.Post("/system/gpu/test", handlers.GPUTest)
			r.Get("/system/gpu/setup", gpuSetupHandler.Status)
			r.Post("/system/gpu/setup/install", gpuSetupHandler.Install)
			r.Post("/system/gpu/setup/uninstall", gpuSetupHandler.Uninstall)
			r.Post("/system/gpu/setup/reboot", gpuSetupHandler.Reboot)
			r.Get("/system/os/status", osUpdateHandler.Status)
			r.Post("/system/os/upgrade", osUpdateHandler.Upgrade)
			r.Post("/system/os/autoremove", osUpdateHandler.Autoremove)
			r.Get("/system/os/search", osUpdateHandler.Search)
			r.Post("/system/os/install", osUpdateHandler.Install)
			r.Get("/system/update/status", selfUpdateHandler.Status)
			r.Post("/system/update", selfUpdateHandler.Update)
			r.Get("/system/tz", systemTZHandler.Status)
			r.Post("/system/tz", systemTZHandler.Apply)
			r.Get("/system/info", systemInfoHandler.Get)
			r.Put("/system/info", systemInfoHandler.Save)

			// Agent proxy — forward actions to inbound agents
			r.HandleFunc("/agent-proxy/{agentId}/*", agentProxyHandler.Proxy)

			// General settings (.env)
			r.Get("/settings/env", envSettingsHandler.Get)
			r.Put("/settings/env", envSettingsHandler.Save)
			r.Post("/settings/env/roll-api-key", envSettingsHandler.RollAPIKey)

			// SSL / TLS settings
			r.Get("/settings/ssl", sslHandler.Get)
			r.Post("/settings/ssl/self-signed", sslHandler.RegenerateSelfSigned)
			r.Post("/settings/ssl/letsencrypt", sslHandler.EnableLetsEncrypt)
			r.Post("/settings/ssl/letsencrypt/renew", sslHandler.RenewLetsEncrypt)

			// Command audit log
			r.Get("/audit", auditHandler.List)
			r.Get("/audit/nodes", auditHandler.Nodes)
			r.Get("/audit/actions", auditHandler.Actions)

			// Skills
			r.Route("/skills", func(sr chi.Router) {
				sr.Get("/", skillHandler.List)
				sr.Get("/{skillName}", skillHandler.Get)
				registry.MountRoutes(sr)
			})
		})
	})

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Stack Manager API starting on %s (root: %s)", addr, cfg.Root)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-done
	log.Println("shutting down...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stopScheduler()
	metricsCollector.Stop()
	updateChecker.Stop()
	registry.ShutdownAll(shutCtx)
	srv.Shutdown(shutCtx)
	log.Println("server stopped")
}

func runAgent(cfg *config.Config, engine *core.Engine) {
	jobs := core.NewJobManager(nil)
	agentHandler := handlers.NewAgentRuntimeHandler(engine, jobs, cfg.AgentToken, cfg.AgentName)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	// Trust X-Real-IP from the built-in nginx container unconditionally.
	// chi's RealIP only trusts loopback, but nginx connects from Docker
	// bridge IPs (172.x.x.x) which chi rejects.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
				r.RemoteAddr = rip + ":0"
			} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				if idx := strings.Index(xff, ","); idx > 0 {
					r.RemoteAddr = strings.TrimSpace(xff[:idx]) + ":0"
				} else {
					r.RemoteAddr = strings.TrimSpace(xff) + ":0"
				}
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Use(chimw.Timeout(5 * time.Minute))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","mode":"agent"}`))
	})

	r.Route("/agent/v1", func(r chi.Router) {
		r.Use(agentHandler.Auth)
		r.Get("/health", agentHandler.Health)
		r.Get("/projects", agentHandler.ListProjects)
		r.Get("/projects/{name}", agentHandler.GetProject)
		r.Get("/projects/{name}/images", agentHandler.Images)
		r.Get("/projects/{name}/status", agentHandler.Status)
		r.Post("/projects/{name}/jobs/{action}", agentHandler.StartJob)
		r.Get("/jobs", agentHandler.ListJobs)
		r.Get("/jobs/{jobId}", agentHandler.GetJob)
		r.Get("/debug/logs/{name}", agentHandler.Logs)
		r.Get("/debug/stats/{name}", agentHandler.Stats)
		r.Post("/registries/login", agentHandler.RegistryLogin)
		r.Get("/registries", agentHandler.ListRegistryLogins)
		r.Delete("/registries/{registry}", agentHandler.DeleteRegistryLogin)
		r.Post("/prune", agentHandler.Prune)
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Stack Manager agent %q starting on %s (root: %s)", cfg.AgentName, addr, cfg.Root)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("agent server: %v", err)
		}
	}()

	<-done
	log.Println("shutting down agent...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutCtx)
	log.Println("agent stopped")
}
