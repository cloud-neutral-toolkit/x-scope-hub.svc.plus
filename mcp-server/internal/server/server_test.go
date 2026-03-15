package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xscopehub/mcp-server/internal/plugins"
	"github.com/xscopehub/mcp-server/internal/registry"
	"github.com/xscopehub/mcp-server/pkg/manifest"
)

func TestServeHTTPResourcesList(t *testing.T) {
	mf := manifest.Manifest{Name: "xscopehub"}
	reg := registry.New()

	// Register Plugins
	obsPlugin := plugins.NewObservabilityPlugin(plugins.ObservabilityPluginConfig{})
	if err := reg.RegisterPlugin(obsPlugin); err != nil {
		t.Fatalf("failed to register observability plugin: %v", err)
	}

	srv := New(Options{Manifest: mf, Registry: reg})

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "resources/list",
		"id":      1,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}

	var resp Response
	if err := json.Unmarshal(res.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error response: %+v", resp.Error)
	}

	if resp.Result == nil {
		t.Fatalf("expected result payload")
	}
}

func TestServeHTTPRequiresBearerTokenWhenConfigured(t *testing.T) {
	mf := manifest.Manifest{Name: "xscopehub"}
	reg := registry.New()
	if err := reg.RegisterPlugin(plugins.NewObservabilityPlugin(plugins.ObservabilityPluginConfig{})); err != nil {
		t.Fatalf("failed to register plugin: %v", err)
	}

	srv := New(Options{Manifest: mf, Registry: reg, AuthToken: "secret-token"})
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "resources/list",
		"id":      1,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	res := httptest.NewRecorder()
	srv.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-token")
	res = httptest.NewRecorder()
	srv.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestServeHTTPManifestEndpoint(t *testing.T) {
	mf := manifest.Manifest{Name: "xscopehub"}
	reg := registry.New()
	if err := reg.RegisterPlugin(plugins.NewObservabilityPlugin(plugins.ObservabilityPluginConfig{})); err != nil {
		t.Fatalf("failed to register plugin: %v", err)
	}

	srv := New(Options{Manifest: mf, Registry: reg})
	req := httptest.NewRequest(http.MethodGet, "/manifest", nil)
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := payload["manifest"]; !ok {
		t.Fatalf("expected manifest payload")
	}
}

func TestServeHTTPHealthzEndpoint(t *testing.T) {
	srv := New(Options{Manifest: manifest.Manifest{Name: "xscopehub"}, Registry: registry.New()})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	srv.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}
