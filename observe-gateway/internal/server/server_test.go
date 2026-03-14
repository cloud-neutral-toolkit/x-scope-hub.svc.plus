package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xscopehub/observe-gateway/internal/audit"
	"github.com/xscopehub/observe-gateway/internal/backend"
	"github.com/xscopehub/observe-gateway/internal/cache"
	"github.com/xscopehub/observe-gateway/internal/config"
	"github.com/xscopehub/observe-gateway/internal/query"
)

type stubBackend struct {
	queryPromQL  func(context.Context, string, query.Request) (backend.Result, error)
	queryLogQL   func(context.Context, string, query.Request) (backend.Result, error)
	queryTraceQL func(context.Context, string, query.Request) (backend.Result, error)
}

func (s stubBackend) QueryPromQL(ctx context.Context, tenant string, req query.Request) (backend.Result, error) {
	if s.queryPromQL == nil {
		return backend.Result{}, nil
	}
	return s.queryPromQL(ctx, tenant, req)
}

func (s stubBackend) QueryLogQL(ctx context.Context, tenant string, req query.Request) (backend.Result, error) {
	if s.queryLogQL == nil {
		return backend.Result{}, nil
	}
	return s.queryLogQL(ctx, tenant, req)
}

func (s stubBackend) QueryTraceQL(ctx context.Context, tenant string, req query.Request) (backend.Result, error) {
	if s.queryTraceQL == nil {
		return backend.Result{}, nil
	}
	return s.queryTraceQL(ctx, tenant, req)
}

func TestHandleQueryResolvesTemplateWithCustomHeaders(t *testing.T) {
	cacheStore, err := cache.New(cache.Config{Enabled: false})
	if err != nil {
		t.Fatalf("cache.New() error = %v", err)
	}

	cfg := config.Config{
		Server: config.ServerConfig{
			TenantHeader: "X-Scope-Tenant",
			UserHeader:   "X-Scope-User",
		},
		QueryTemplates: map[string]config.QueryTemplateConfig{
			"service_error_logs": {
				Lang:  "logql",
				Query: `{service="{{service}}"} |= "error"`,
			},
		},
	}

	called := false
	srv := &Server{
		cfg:   cfg,
		cache: cacheStore,
		backend: stubBackend{
			queryLogQL: func(_ context.Context, tenant string, req query.Request) (backend.Result, error) {
				called = true
				if tenant != "tenant-a" {
					t.Fatalf("tenant = %q, want tenant-a", tenant)
				}
				if req.Lang != "logql" {
					t.Fatalf("lang = %q, want logql", req.Lang)
				}
				if req.Query != `{service="api"} |= "error"` {
					t.Fatalf("query = %q", req.Query)
				}
				if req.Start.IsZero() || req.End.IsZero() {
					t.Fatalf("expected time range to be passed through")
				}
				return backend.Result{
					Payload: json.RawMessage(`{"ok":true}`),
					Backend: "stub-logql",
					Cost:    7,
				}, nil
			},
		},
		auditLog: audit.New(false, nil),
	}

	reqBody, err := json.Marshal(query.Request{
		Template:  "service_error_logs",
		Variables: map[string]string{"service": "api"},
		Start:     time.Now().Add(-15 * time.Minute).UTC(),
		End:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/query", bytes.NewReader(reqBody))
	req.Header.Set("X-Scope-Tenant", "tenant-a")
	req.Header.Set("X-Scope-User", "user-a")
	rec := httptest.NewRecorder()

	srv.handleQuery(rec, req)

	if !called {
		t.Fatal("expected backend to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp query.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.Tenant != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", resp.Tenant)
	}
	if resp.Stats.Backend != "stub-logql" {
		t.Fatalf("backend = %q, want stub-logql", resp.Stats.Backend)
	}
}

func TestResolveTemplateLeavesExplicitQueryIntact(t *testing.T) {
	srv := &Server{
		cfg: config.Config{
			QueryTemplates: map[string]config.QueryTemplateConfig{
				"service_error_rate": {
					Lang:  "promql",
					Query: `sum(rate(errors_total{service="{{service}}"}[{{window}}]))`,
					Step:  "1m",
				},
			},
		},
	}

	req := srv.resolveTemplate(query.Request{
		Lang:      "promql",
		Query:     `up{service="payments"}`,
		Template:  "service_error_rate",
		Variables: map[string]string{"service": "payments", "window": "5m"},
		Step:      "30s",
	})

	if req.Query != `up{service="payments"}` {
		t.Fatalf("query = %q, want explicit query preserved", req.Query)
	}
	if req.Step != "30s" {
		t.Fatalf("step = %q, want explicit step preserved", req.Step)
	}
}
