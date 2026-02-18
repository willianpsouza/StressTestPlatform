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

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/handlers"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/middleware"
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

	// Repositories
	userRepo := postgres.NewUserRepository(dbPool)
	sessionRepo := postgres.NewSessionRepository(dbPool)

	// Cache
	_ = redisadapter.NewCache(redisClient)

	// Services
	authService := app.NewAuthService(cfg.JWT, userRepo, sessionRepo)

	// Handlers
	healthHandler := handlers.NewHealthHandler(dbPool, redisClient, cfg)
	authHandler := handlers.NewAuthHandler(authService)

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
