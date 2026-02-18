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

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/grafana"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/handlers"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/middleware"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/influxdb"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/postgres"
	redisadapter "github.com/willianpsouza/StressTestPlatform/internal/adapters/redis"
	"github.com/willianpsouza/StressTestPlatform/internal/app"
	"github.com/willianpsouza/StressTestPlatform/internal/pkg/config"
)

func main() {
	cfg := config.Load()

	log.Printf("Starting %s (env=%s, project=%s)", cfg.App.Name, cfg.App.Env, cfg.App.ProjectName)

	// PostgreSQL
	dbPool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Redis
	redisOpts, err := goredis.ParseURL(cfg.Redis.URL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	redisClient := goredis.NewClient(redisOpts)
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()
	log.Println("Connected to Redis")

	// External clients
	influxClient := influxdb.NewClient(cfg.InfluxDB)
	grafanaClient := grafana.NewClient(cfg.Grafana)
	_ = grafanaClient

	// Repositories
	userRepo := postgres.NewUserRepository(dbPool)
	sessionRepo := postgres.NewSessionRepository(dbPool)
	domainRepo := postgres.NewDomainRepository(dbPool)
	testRepo := postgres.NewTestRepository(dbPool)
	execRepo := postgres.NewExecutionRepository(dbPool)
	scheduleRepo := postgres.NewScheduleRepository(dbPool)

	// Cache
	_ = redisadapter.NewCache(redisClient)

	// K6 Runner
	k6Runner := app.NewK6Runner(execRepo, testRepo, influxClient.URL(), influxClient.Token(), influxClient.Org(), cfg.K6)
	k6Runner.RecoverOrphans()

	// Services
	authService := app.NewAuthService(cfg.JWT, userRepo, sessionRepo)
	domainService := app.NewDomainService(domainRepo)
	testService := app.NewTestService(testRepo, domainRepo, influxClient, cfg.K6)
	execService := app.NewExecutionService(execRepo, testRepo, k6Runner)
	scheduleService := app.NewScheduleService(scheduleRepo, testRepo)

	// Scheduler
	scheduler := app.NewScheduler(scheduleRepo, execRepo, k6Runner)
	scheduler.Start()

	// Handlers
	healthHandler := handlers.NewHealthHandler(dbPool, redisClient, cfg)
	authHandler := handlers.NewAuthHandler(authService)
	domainHandler := handlers.NewDomainHandler(domainService)
	testHandler := handlers.NewTestHandler(testService)
	execHandler := handlers.NewExecutionHandler(execService)
	dashboardHandler := handlers.NewDashboardHandler(execService)
	scheduleHandler := handlers.NewScheduleHandler(scheduleService)
	influxdbHandler := handlers.NewInfluxDBHandler(influxClient, testRepo)

	// Router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health endpoints
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth routes (rate limited)
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(30, 1*time.Minute))
			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
			r.Post("/auth/refresh", authHandler.Refresh)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(authService))

			// Auth
			r.Post("/auth/logout", authHandler.Logout)
			r.Get("/auth/me", authHandler.Me)
			r.Put("/auth/me", authHandler.UpdateProfile)
			r.Post("/auth/change-password", authHandler.ChangePassword)

			// Domains
			r.Get("/domains", domainHandler.List)
			r.Post("/domains", domainHandler.Create)
			r.Get("/domains/{id}", domainHandler.Get)
			r.Put("/domains/{id}", domainHandler.Update)
			r.Delete("/domains/{id}", domainHandler.Delete)

			// Tests
			r.Get("/tests", testHandler.List)
			r.Post("/tests", testHandler.Create)
			r.Get("/tests/{id}", testHandler.Get)
			r.Put("/tests/{id}", testHandler.Update)
			r.Put("/tests/{id}/script", testHandler.UpdateScript)
			r.Delete("/tests/{id}", testHandler.Delete)

			// Executions
			r.Get("/executions", execHandler.List)
			r.Post("/executions", execHandler.Create)
			r.Get("/executions/{id}", execHandler.Get)
			r.Post("/executions/{id}/cancel", execHandler.Cancel)
			r.Get("/executions/{id}/logs", execHandler.Logs)

			// Schedules
			r.Get("/schedules", scheduleHandler.List)
			r.Post("/schedules", scheduleHandler.Create)
			r.Get("/schedules/{id}", scheduleHandler.Get)
			r.Put("/schedules/{id}", scheduleHandler.Update)
			r.Delete("/schedules/{id}", scheduleHandler.Delete)
			r.Post("/schedules/{id}/pause", scheduleHandler.Pause)
			r.Post("/schedules/{id}/resume", scheduleHandler.Resume)

			// Dashboard (all users see all executions)
			r.Get("/dashboard/executions", dashboardHandler.ListExecutions)
			r.Get("/dashboard/stats", dashboardHandler.Stats)

			// InfluxDB bucket management
			r.Get("/influxdb/buckets", influxdbHandler.ListBuckets)
			r.Post("/influxdb/buckets/{name}/clear", influxdbHandler.ClearBucket)

			// ROOT-only: user management
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("ROOT"))
				r.Get("/users", authHandler.ListUsers)
				r.Get("/users/{id}", authHandler.GetUser)
				r.Put("/users/{id}", authHandler.UpdateUser)
				r.Delete("/users/{id}", authHandler.DeleteUser)
			})
		})
	})

	// Server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down server...")

	scheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
