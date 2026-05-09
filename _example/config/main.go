// Config example: typed environment configuration via LoadEnvConfig[T].
//
// Unlike LoadEnv (which uses caarlos0/env tags), LoadEnvConfig is built on
// koanf and supports nested structures: an underscore in the env key becomes
// a level of nesting, so DEMO_API_SERVER_PORT maps to server.port. Values
// containing spaces are split into slices.
//
// Run with:
//
//	go run ./_example/config
//
// Override config via env (prefix = "DEMO_API_"):
//
//	DEMO_API_SERVER_HOST=0.0.0.0 \
//	DEMO_API_SERVER_PORT=9090 \
//	DEMO_API_DATABASE_URL=postgres://user:pass@localhost/db \
//	DEMO_API_TAGS="alpha beta gamma" \
//	go run ./_example/config
//
// Graceful shutdown: Ctrl-C (SIGINT) or `kill -TERM <pid>`.
package main

import (
	"context"
	"fmt"

	"go.aledante.io/as"
)

func main() {
	as.RunAndExit(
		&apiService{},
		as.WithLogJson(false),
		as.WithLogDebug(true),
	)
}

// Config is filled by LoadEnvConfig[Config] from prefixed environment
// variables. The default struct tag is `config`; pass WithConfigTag to use a
// different one. Underscores in env keys are interpreted as nesting, so with
// prefix "DEMO_API_":
//
//	DEMO_API_SERVER_PORT=9090       → Config.Server.Port
//	DEMO_API_DATABASE_URL=...       → Config.Database.URL
//	DEMO_API_TAGS="alpha beta"      → Config.Tags = []string{"alpha", "beta"}
type Config struct {
	Server   ServerConfig   `config:"server"`
	Database DatabaseConfig `config:"database"`
	Tags     []string       `config:"tags"`
}

type ServerConfig struct {
	Host string `config:"host"`
	Port int    `config:"port"`
}

type DatabaseConfig struct {
	URL string `config:"url"`
}

type apiService struct {
	cfg *Config
}

func (s *apiService) Name() string      { return "api" }
func (s *apiService) Namespace() string { return "demo" }
func (s *apiService) Version() string   { return "1.0.0" }

// Init loads configuration from the environment via LoadEnvConfig. Runs on
// every (init → run → close) iteration, so it must be idempotent.
func (s *apiService) Init(ctx context.Context) error {
	cfg, err := as.LoadEnvConfig[Config](ctx)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	s.cfg = cfg

	as.Logger(ctx).Info("configuration loaded",
		"env_prefix", as.EnvPrefix(ctx),
		"server.host", cfg.Server.Host,
		"server.port", cfg.Server.Port,
		"database.url", cfg.Database.URL,
		"tags", cfg.Tags,
	)
	return nil
}

// Run blocks until the run context is cancelled by a signal.
func (s *apiService) Run(ctx context.Context) error {
	as.Logger(ctx).Info("service running",
		"host", s.cfg.Server.Host,
		"port", s.cfg.Server.Port,
	)
	<-ctx.Done()
	return ctx.Err()
}

func (s *apiService) Close(ctx context.Context) error {
	as.Logger(ctx).Info("closing service")
	return nil
}
