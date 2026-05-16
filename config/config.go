package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	BindAddr        string
	DBPath          string
	AdminSecret     string
	DefaultModel    string
	RateLimitRPM    int
	UpstreamURL     string
	UpstreamModels  string
	UpstreamStatus  string
	AtomCodeConfDir string
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:            envOrDefault("PORT", "8080"),
		BindAddr:        envOrDefault("BIND_ADDR", "0.0.0.0"),
		DBPath:          envOrDefault("DB_PATH", "data/atom2api.db"),
		AdminSecret:     os.Getenv("ADMIN_SECRET"),
		DefaultModel:    envOrDefault("DEFAULT_MODEL", "deepseek-v4-flash"),
		UpstreamURL:     envOrDefault("UPSTREAM_URL", "https://api-ai.gitcode.com/v1/chat/completions"),
		UpstreamModels:  envOrDefault("UPSTREAM_MODELS_URL", "https://api.gitcode.com/api/v5/coding-plan/models"),
		UpstreamStatus:  envOrDefault("UPSTREAM_STATUS_URL", "https://api.gitcode.com/api/v5/coding-plan/status"),
		AtomCodeConfDir: envOrDefault("ATOMCODE_DIR", defaultAtomCodeDir()),
	}
	return cfg
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func defaultAtomCodeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".atomcode")
}
