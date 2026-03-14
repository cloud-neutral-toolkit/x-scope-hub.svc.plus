package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeReasoner struct {
	diagnosis Diagnosis
	err       error
}

func (f fakeReasoner) Analyze(context.Context, ReasonerInput) (Diagnosis, error) {
	if f.err != nil {
		return Diagnosis{}, f.err
	}
	return f.diagnosis, nil
}

func TestRunFallsBackWhenReasonerFails(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		template := body["template"].(string)
		w.Header().Set("Content-Type", "application/json")
		switch template {
		case templateErrorRate, templateLatencyP95:
			_, _ = w.Write([]byte(`{"lang":"promql","tenant":"tenant-a","result":{"data":{"result":[{"metric":{"service":"checkout"}}]}},"stats":{"backend":"mock","cached":false,"duration_ms":2,"cost":1}}`))
		case templateErrorLogs:
			_, _ = w.Write([]byte(`{"lang":"logql","tenant":"tenant-a","result":{"hits":[{"message":"error one"},{"message":"error two"}]},"stats":{"backend":"mock","cached":false,"duration_ms":2,"cost":1}}`))
		case templateErrorTraces:
			_, _ = w.Write([]byte(`{"lang":"traceql","tenant":"tenant-a","result":{"hits":[{"trace_id":"t1"}]},"stats":{"backend":"mock","cached":false,"duration_ms":2,"cost":1}}`))
		default:
			_, _ = w.Write([]byte(`{"lang":"logql","tenant":"tenant-a","result":{"hits":[{"dependency":"payments"}]},"stats":{"backend":"mock","cached":false,"duration_ms":2,"cost":1}}`))
		}
	}))
	defer gateway.Close()

	svc, err := NewService(Options{
		Gateway: GatewayOptions{
			Endpoint: gateway.URL,
		},
		Reasoner:      fakeReasoner{err: errors.New("codex unavailable")},
		DefaultTenant: "tenant-a",
		DefaultWindow: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	resp, err := svc.Run(context.Background(), Request{
		Service:  "checkout",
		Incident: "error rate increased",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if resp.Meta.ReasonerStatus != "fallback" {
		t.Fatalf("reasoner status = %q, want fallback", resp.Meta.ReasonerStatus)
	}
	if resp.Diagnosis.Source != "heuristic-fallback" {
		t.Fatalf("diagnosis source = %q, want heuristic-fallback", resp.Diagnosis.Source)
	}
	if resp.Evidence.Logs.Count != 2 {
		t.Fatalf("logs count = %d, want 2", resp.Evidence.Logs.Count)
	}
	if resp.Evidence.Traces.Count != 1 {
		t.Fatalf("traces count = %d, want 1", resp.Evidence.Traces.Count)
	}
	if len(resp.Meta.Errors) == 0 {
		t.Fatal("expected reasoner error to be surfaced")
	}
}

func TestRunUsesReasonerResponseWhenAvailable(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"lang":"promql","tenant":"tenant-a","result":{"data":{"result":[]}},"stats":{"backend":"mock","cached":false,"duration_ms":2,"cost":1}}`))
	}))
	defer gateway.Close()

	svc, err := NewService(Options{
		Gateway: GatewayOptions{
			Endpoint: gateway.URL,
		},
		Reasoner: fakeReasoner{
			diagnosis: Diagnosis{
				Summary:      "codex summary",
				LikelyCauses: []string{"dependency outage"},
				Evidence:     []string{"trace fanout failed"},
				Remediation:  []string{"rollback latest deploy"},
				Confidence:   0.9,
				Source:       "codex",
			},
		},
		DefaultTenant: "tenant-a",
		DefaultWindow: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	resp, err := svc.Run(context.Background(), Request{Service: "checkout"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if resp.Meta.ReasonerStatus != "completed" {
		t.Fatalf("reasoner status = %q, want completed", resp.Meta.ReasonerStatus)
	}
	if resp.Diagnosis.Summary != "codex summary" {
		t.Fatalf("summary = %q", resp.Diagnosis.Summary)
	}
}
