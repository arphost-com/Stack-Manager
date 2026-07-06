package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cmauth "github.com/arphost-com/Stack-Manager/server/internal/auth"
	"github.com/arphost-com/Stack-Manager/server/internal/config"
	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/arphost-com/Stack-Manager/server/internal/handlers"
	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
	"github.com/arphost-com/Stack-Manager/server/internal/skills"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/backup"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/dbadmin"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/debug"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/frontend"
	"github.com/arphost-com/Stack-Manager/server/internal/skills/security"
	"github.com/arphost-com/Stack-Manager/server/internal/storage"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Core engine
	engine := core.NewEngine(cfg.Root, cfg.HooksDir)
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

	jobs := core.NewJobManager(appStore)
	scheduler := core.NewScheduleManager(engine, jobs, appStore)
	metricsCollector := core.NewMetricsCollector(engine, appStore, cfg.MetricsInterval, cfg.WarmCacheTTL)
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	scheduler.Start(schedulerCtx)
	metricsCollector.Start(schedulerCtx)
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
	agentHandler := handlers.NewAgentHandler(appStore)
	scheduleHandler := handlers.NewScheduleHandler(appStore, scheduler)
	metricsHandler := handlers.NewMetricsHandler(appStore, metricsCollector)
	dockerSettingsHandler := handlers.NewDockerSettingsHandler(cfg.DockerDaemonDir, cfg.BaseImagePrefix)
	sslHandler := handlers.NewSSLHandler(cfg.StateDir, cfg.BaseImagePrefix)
	auditHandler := handlers.NewAuditHandler(appStore)
	skillHandler := handlers.NewSkillHandler(registry)
	authHandler := handlers.NewAuthHandler(userStore, sessionManager)

	// Router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
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

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(cfg.APIKey, sessionManager))
			r.Use(middleware.AuditRecorder(appStore, os.Getenv("AUDIT_NODE_NAME")))

			r.Get("/auth/me", authHandler.Me)
			r.Post("/auth/logout", authHandler.Logout)
			r.Get("/users", authHandler.ListUsers)
			r.Post("/users", authHandler.CreateUser)
			r.Put("/users/{username}/password", authHandler.SetPassword)
			r.Delete("/users/{username}", authHandler.DeleteUser)

			// Project endpoints
			r.Post("/projects", projectHandler.Create)
			r.Get("/projects", projectHandler.List)
			r.Get("/projects/{name}", projectHandler.Get)
			r.Delete("/projects/{name}", projectHandler.Delete)
			r.Get("/projects/{name}/images", projectHandler.Images)
			r.Get("/projects/{name}/status", projectHandler.Status)
			r.Post("/projects/{name}/pull", projectHandler.Pull)
			r.Post("/projects/{name}/up", projectHandler.Up)
			r.Post("/projects/{name}/down", projectHandler.Down)
			r.Post("/projects/{name}/update", projectHandler.Update)
			r.Post("/projects/{name}/restart", projectHandler.Restart)
			r.Post("/projects/{name}/jobs/{action}", projectHandler.StartJob)
			r.Get("/projects/{name}/update-policy", projectHandler.GetUpdatePolicy)
			r.Put("/projects/{name}/update-policy", projectHandler.SetUpdatePolicy)
			r.Put("/projects/{name}/inactive", projectHandler.SetInactive)

			// Bulk operations
			r.Post("/projects/bulk/{action}", projectHandler.BulkAction)

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
	r.Use(chimw.RealIP)
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
