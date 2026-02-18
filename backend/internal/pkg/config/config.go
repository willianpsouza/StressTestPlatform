package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	InfluxDB InfluxDBConfig
	Grafana  GrafanaConfig
	K6       K6Config
}

type AppConfig struct {
	Env         string
	Name        string
	Debug       bool
	ProjectName string
}

type ServerConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	URL      string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret               string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
}

type InfluxDBConfig struct {
	URL   string
	Token string
	Org   string
}

type GrafanaConfig struct {
	URL           string
	PublicURL     string
	AdminUser     string
	AdminPassword string
}

type K6Config struct {
	MaxDuration   time.Duration
	MaxVUs        int
	MaxConcurrent int
	ScriptsPath   string
}

func Load() *Config {
	return &Config{
		App: AppConfig{
			Env:         getEnv("APP_ENV", "development"),
			Name:        getEnv("APP_NAME", "StressTestPlatform"),
			Debug:       getEnvBool("APP_DEBUG", true),
			ProjectName: getEnv("PROJECT_NAME", "BR-IDNF"),
		},
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			URL:             getEnv("DATABASE_URL", "postgres://stresstest:stresstest_secret@localhost:5432/stresstest?sslmode=disable"),
			MaxOpenConns:    getEnvInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", "redis://localhost:6379/0"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:               getEnv("JWT_SECRET", "dev-secret-change-in-production"),
			AccessTokenDuration:  getEnvDuration("JWT_ACCESS_TOKEN_DURATION", 15*time.Minute),
			RefreshTokenDuration: getEnvDuration("JWT_REFRESH_TOKEN_DURATION", 7*24*time.Hour),
		},
		InfluxDB: InfluxDBConfig{
			URL:   getEnv("INFLUXDB_URL", "http://localhost:8086"),
			Token: getEnv("INFLUXDB_TOKEN", ""),
			Org:   getEnv("INFLUXDB_ORG", "stresstest"),
		},
		Grafana: GrafanaConfig{
			URL:           getEnv("GRAFANA_URL", "http://localhost:3001"),
			PublicURL:     getEnv("GRAFANA_PUBLIC_URL", "/grafana"),
			AdminUser:     getEnv("GRAFANA_ADMIN_USER", "admin"),
			AdminPassword: getEnv("GRAFANA_ADMIN_PASSWORD", "admin"),
		},
		K6: K6Config{
			MaxDuration:   getEnvDuration("K6_MAX_DURATION", 5*time.Minute),
			MaxVUs:        getEnvInt("K6_MAX_VUS", 20),
			MaxConcurrent: getEnvInt("K6_MAX_CONCURRENT", 5),
			ScriptsPath:   getEnv("K6_SCRIPTS_PATH", "/app/k6-scripts"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
