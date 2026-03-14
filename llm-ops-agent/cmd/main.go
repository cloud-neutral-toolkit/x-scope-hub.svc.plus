package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	daemon "github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/yourname/XOpsAgent/api"
	"github.com/yourname/XOpsAgent/internal/analysis"
	"github.com/yourname/XOpsAgent/internal/ports/postgres"
	"github.com/yourname/XOpsAgent/internal/services/orchestrator"
)

type Config struct {
	Server struct {
		API struct {
			Listen         string `yaml:"listen"`
			ResponseFormat string `yaml:"response_format"`
		} `yaml:"api"`
	} `yaml:"server"`
	Inputs struct {
		Postgres struct {
			URL string `yaml:"url"`
		} `yaml:"postgres"`
		OpenObserve struct {
			Endpoint string            `yaml:"endpoint"`
			Headers  map[string]string `yaml:"headers"`
		} `yaml:"openobserve"`
		ObserveGateway struct {
			Endpoint      string            `yaml:"endpoint"`
			Headers       map[string]string `yaml:"headers"`
			TenantHeader  string            `yaml:"tenant_header"`
			UserHeader    string            `yaml:"user_header"`
			DefaultTenant string            `yaml:"default_tenant"`
			DefaultUser   string            `yaml:"default_user"`
			Timeout       time.Duration     `yaml:"timeout"`
		} `yaml:"observe_gateway"`
	} `yaml:"inputs"`
	Models struct {
		Embedder struct {
			Name     string `yaml:"name"`
			Endpoint string `yaml:"endpoint"`
		} `yaml:"embedder"`
		Generator struct {
			Models   []string `yaml:"models"`
			Endpoint string   `yaml:"endpoint"`
		} `yaml:"generator"`
		Codex struct {
			Enabled    bool              `yaml:"enabled"`
			Command    string            `yaml:"command"`
			Args       []string          `yaml:"args"`
			WorkingDir string            `yaml:"working_dir"`
			Timeout    time.Duration     `yaml:"timeout"`
			Env        map[string]string `yaml:"env"`
		} `yaml:"codex"`
	} `yaml:"models"`
	Outputs struct {
		GitHubPR struct {
			Enabled  bool   `yaml:"enabled"`
			Repo     string `yaml:"repo"`
			TokenEnv string `yaml:"token_env"`
		} `yaml:"github_pr"`
		FileReport struct {
			Enabled bool   `yaml:"enabled"`
			Path    string `yaml:"path"`
			Format  string `yaml:"format"`
		} `yaml:"file_report"`
		Answer struct {
			Enabled bool   `yaml:"enabled"`
			Channel string `yaml:"channel"`
		} `yaml:"answer"`
		Webhook struct {
			Enabled bool              `yaml:"enabled"`
			URL     string            `yaml:"url"`
			Headers map[string]string `yaml:"headers"`
		} `yaml:"webhook"`
	} `yaml:"outputs"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	expanded := os.ExpandEnv(string(data))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func runAgent(cfgPath string) error {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logConnections(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var sqlDB *sql.DB
	var pool *pgxpool.Pool
	var caseSvc orchestrator.Service
	if cfg.Inputs.Postgres.URL != "" {
		sqlDB, pool, caseSvc, err = newCaseService(ctx, cfg.Inputs.Postgres.URL)
		if err != nil {
			return fmt.Errorf("init case service: %w", err)
		}
		defer sqlDB.Close()
		defer pool.Close()
	}

	analysisSvc, err := newAnalysisService(cfg, cfgPath)
	if err != nil {
		return fmt.Errorf("init analysis service: %w", err)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	api.RegisterRoutes(r, caseSvc)
	api.RegisterAnalysisRoutes(r, analysisSvc)

	listen := cfg.Server.API.Listen
	if listen == "" {
		listen = "0.0.0.0:8100"
	}

	srv := &http.Server{
		Addr:              listen,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("XOpsAgent daemon listening on %s", listen)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func newCaseService(ctx context.Context, databaseURL string) (*sql.DB, *pgxpool.Pool, orchestrator.Service, error) {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, nil, nil, err
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		sqlDB.Close()
		return nil, nil, nil, err
	}

	repo := postgres.NewCaseRepository(pool)
	return sqlDB, pool, orchestrator.New(repo), nil
}

func newAnalysisService(cfg *Config, cfgPath string) (analysis.Service, error) {
	gatewayCfg := cfg.Inputs.ObserveGateway
	if gatewayCfg.Endpoint == "" {
		gatewayCfg.Endpoint = cfg.Inputs.OpenObserve.Endpoint
	}
	if len(gatewayCfg.Headers) == 0 {
		gatewayCfg.Headers = cfg.Inputs.OpenObserve.Headers
	}

	var reasoner analysis.Reasoner
	if cfg.Models.Codex.Enabled || cfg.Models.Codex.Command != "" {
		command := cfg.Models.Codex.Command
		if command == "" {
			command = filepath.Join(repoRootFromConfig(cfgPath), "scripts/codex/run-monitor.sh")
		}
		reasoner = analysis.NewCodexReasoner(analysis.CodexReasonerConfig{
			Command:    command,
			Args:       cfg.Models.Codex.Args,
			WorkingDir: defaultWorkingDir(cfg.Models.Codex.WorkingDir, cfgPath),
			Timeout:    cfg.Models.Codex.Timeout,
			Env:        cfg.Models.Codex.Env,
		})
	}

	return analysis.NewService(analysis.Options{
		Gateway: analysis.GatewayOptions{
			Endpoint:     gatewayCfg.Endpoint,
			Headers:      gatewayCfg.Headers,
			TenantHeader: gatewayCfg.TenantHeader,
			UserHeader:   gatewayCfg.UserHeader,
			Timeout:      gatewayCfg.Timeout,
		},
		Reasoner:      reasoner,
		DefaultTenant: gatewayCfg.DefaultTenant,
		DefaultUser:   gatewayCfg.DefaultUser,
		DefaultWindow: time.Hour,
		MaxItems:      50,
	})
}

func repoRootFromConfig(cfgPath string) string {
	abs, err := filepath.Abs(cfgPath)
	if err != nil {
		return "."
	}
	return filepath.Dir(filepath.Dir(abs))
}

func defaultWorkingDir(workingDir string, cfgPath string) string {
	if workingDir != "" {
		return workingDir
	}
	return repoRootFromConfig(cfgPath)
}

func logConnections(cfg *Config) {
	logger := slog.Default()
	checkPostgres(logger, cfg.Inputs.Postgres.URL)
	if cfg.Inputs.ObserveGateway.Endpoint != "" {
		checkHTTP(logger, "inputs.observe_gateway", cfg.Inputs.ObserveGateway.Endpoint, cfg.Inputs.ObserveGateway.Headers)
	} else {
		checkHTTP(logger, "inputs.openobserve", cfg.Inputs.OpenObserve.Endpoint, cfg.Inputs.OpenObserve.Headers)
	}
	checkHTTP(logger, "models.embedder", cfg.Models.Embedder.Endpoint, nil)
	checkHTTP(logger, "models.generator", cfg.Models.Generator.Endpoint, nil)

	if cfg.Outputs.GitHubPR.Enabled {
		checkGitHub(logger, cfg.Outputs.GitHubPR.Repo, cfg.Outputs.GitHubPR.TokenEnv)
	}
	if cfg.Outputs.FileReport.Enabled {
		checkFilePath(logger, cfg.Outputs.FileReport.Path)
	}
	if cfg.Outputs.Answer.Enabled {
		logger.Info("outputs.answer configured", "channel", cfg.Outputs.Answer.Channel)
	}
	if cfg.Outputs.Webhook.Enabled {
		checkHTTP(logger, "outputs.webhook", cfg.Outputs.Webhook.URL, cfg.Outputs.Webhook.Headers)
	}
}

func checkPostgres(logger *slog.Logger, url string) {
	if url == "" {
		logger.Debug("inputs.postgres not configured")
		return
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		logger.Error("inputs.postgres connection", "error", err)
		return
	}
	if err := db.Ping(); err != nil {
		logger.Warn("inputs.postgres ping failed", "error", err)
	} else {
		logger.Info("inputs.postgres reachable")
	}
	_ = db.Close()
}

func checkHTTP(logger *slog.Logger, name, url string, headers map[string]string) {
	if url == "" {
		logger.Debug(name + " not configured")
		return
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(name+" request", "endpoint", url, "error", err)
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	logger.Info("request", "target", name, "endpoint", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn(name+" unreachable", "endpoint", url, "error", err)
		return
	}
	resp.Body.Close()
	logger.Info(name+" reachable", "endpoint", url, "status", resp.StatusCode)
}

func checkFilePath(logger *slog.Logger, path string) {
	if path == "" {
		logger.Debug("outputs.file_report not configured")
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Warn("outputs.file_report inaccessible", "path", path, "error", err)
	} else {
		logger.Info("outputs.file_report path ok", "path", path)
	}
}

func checkGitHub(logger *slog.Logger, repo, tokenEnv string) {
	if repo == "" {
		logger.Debug("outputs.github_pr repo not configured")
		return
	}
	token := os.Getenv(tokenEnv)
	headers := map[string]string{}
	if token == "" {
		logger.Warn("outputs.github_pr token missing", "env", tokenEnv)
	} else {
		headers["Authorization"] = "Bearer " + token
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s", repo)
	checkHTTP(logger, "outputs.github_pr", url, headers)
}

var (
	daemonMode bool
	cfgPath    string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "llm-ops-agent",
		Short: "LLM Ops Agent service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonMode {
				cntxt := &daemon.Context{
					PidFileName: "xopsagent.pid",
					PidFilePerm: 0o644,
				}
				child, err := cntxt.Reborn()
				if err != nil {
					return err
				}
				if child != nil {
					return nil
				}
				defer cntxt.Release()
			}
			return runAgent(cfgPath)
		},
	}
	rootCmd.PersistentFlags().BoolVar(&daemonMode, "daemon", true, "run in background")
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "/etc/XOpsAgent.yaml", "path to config file")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
