package plugins

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryMetricsUsesObserveGateway(t *testing.T) {
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/api/query" {
			t.Fatalf("path = %s, want /api/query", r.URL.Path)
		}
		if got := r.Header.Get("X-Tenant"); got != "tenant-a" {
			t.Fatalf("tenant header = %q, want tenant-a", got)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["template"] != "service_error_rate" && body["template"] != "service_latency_p95" {
			t.Fatalf("unexpected template %v", body["template"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"lang":"promql","tenant":"tenant-a","result":{"data":{"result":[{"metric":{"service":"checkout"}}]}},"stats":{"backend":"mock","cached":false,"duration_ms":3,"cost":1}}`))
	}))
	defer upstream.Close()

	plugin := NewObservabilityPlugin(ObservabilityPluginConfig{
		ObserveGatewayURL: upstream.URL,
		DefaultTenant:     "tenant-a",
		TenantHeader:      "X-Tenant",
		UserHeader:        "X-User",
	})

	result, err := plugin.ExecuteTool("query_metrics", map[string]interface{}{
		"service": "checkout",
		"metric":  "error_rate",
	})
	if err != nil {
		t.Fatalf("ExecuteTool() error = %v", err)
	}
	if !called {
		t.Fatal("expected observe-gateway to be called")
	}
	if result.Name != "query_metrics" {
		t.Fatalf("result.Name = %q", result.Name)
	}
}

func TestAnalyzeIncidentUsesAgentEndpoint(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/analysis/run" {
			t.Fatalf("path = %s, want /analysis/run", r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["goal"] != "analyze_incident" {
			t.Fatalf("goal = %v, want analyze_incident", body["goal"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"goal":"analyze_incident","tenant":"tenant-a","service":"checkout","health":{"status":"degraded","highlights":["log evidence captured 3 record(s)"]},"diagnosis":{"summary":"summary","likely_causes":["cause"],"evidence":["e1"],"remediation":["r1"],"confidence":0.8,"source":"codex"},"meta":{"reasoner":"codex","reasoner_status":"completed"}}`))
	}))
	defer agent.Close()

	plugin := NewObservabilityPlugin(ObservabilityPluginConfig{
		LlmOpsAgentURL: agent.URL,
		DefaultTenant:  "tenant-a",
	})

	result, err := plugin.ExecuteTool("analyze_incident", map[string]interface{}{
		"service":  "checkout",
		"incident": "error rate increased",
	})
	if err != nil {
		t.Fatalf("ExecuteTool() error = %v", err)
	}
	output, ok := result.Output.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected output type %T", result.Output)
	}
	if output["service"] != "checkout" {
		t.Fatalf("service = %v, want checkout", output["service"])
	}
}
