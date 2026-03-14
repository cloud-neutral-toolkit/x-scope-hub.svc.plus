package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/xscopehub/mcp-server/internal/plugins"
	"github.com/xscopehub/mcp-server/internal/registry"
	"github.com/xscopehub/mcp-server/internal/server"
	"github.com/xscopehub/mcp-server/pkg/manifest"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "serve":
		serve(os.Args[2:])
	case "manifest":
		printManifest(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <serve|manifest> [flags]\n", filepath.Base(os.Args[0]))
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", getenv("XSCOPE_MCP_LISTEN_ADDR", ":8000"), "Address to listen on")
	manifestPath := fs.String("manifest", getenv("XSCOPE_MCP_SERVER_MANIFEST", "manifest.json"), "Path to manifest file")
	readTimeout := fs.Duration("read-timeout", 5*time.Second, "HTTP server read timeout")
	writeTimeout := fs.Duration("write-timeout", 10*time.Second, "HTTP server write timeout")
	authToken := fs.String("auth-token", os.Getenv("XSCOPE_MCP_SERVER_AUTH_TOKEN"), "Optional bearer token required by /mcp")
	_ = fs.Parse(args)

	mf, err := manifest.Load(*manifestPath)
	if err != nil {
		log.Fatalf("failed to load manifest: %v", err)
	}

	reg := registry.New()

	// Register Plugins
	obsPlugin := plugins.NewObservabilityPlugin(plugins.ObservabilityPluginConfig{
		ObserveGatewayURL: getenv("XSCOPE_OBSERVE_GATEWAY_URL", "http://127.0.0.1:8080"),
		LlmOpsAgentURL:    getenv("XSCOPE_LLM_OPS_AGENT_URL", "http://127.0.0.1:8100"),
		DefaultTenant:     os.Getenv("XSCOPE_DEFAULT_TENANT"),
		DefaultUser:       os.Getenv("XSCOPE_DEFAULT_USER"),
		TenantHeader:      getenv("OBSERVE_GATEWAY_TENANT_HEADER", "X-Tenant"),
		UserHeader:        getenv("OBSERVE_GATEWAY_USER_HEADER", "X-User"),
		Timeout:           durationFromEnv("XSCOPE_MCP_UPSTREAM_TIMEOUT", 20*time.Second),
	})
	if err := reg.RegisterPlugin(obsPlugin); err != nil {
		log.Fatalf("failed to register observability plugin: %v", err)
	}

	srv := server.New(server.Options{
		Manifest:     mf,
		Registry:     reg,
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
		AuthToken:    strings.TrimSpace(*authToken),
	})

	httpSrv := &http.Server{
		Addr:         *addr,
		Handler:      srv,
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
	}

	go func() {
		log.Printf("mcp server listening on %s", *addr)
		if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func printManifest(args []string) {
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	manifestPath := fs.String("manifest", "manifest.json", "Path to manifest file")
	_ = fs.Parse(args)

	mf, err := manifest.Load(*manifestPath)
	if err != nil {
		log.Fatalf("failed to load manifest: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(mf); err != nil {
		log.Fatalf("failed to encode manifest: %v", err)
	}
}

func getenv(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
