package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/yourname/XOpsAgent/internal/analysis"
)

type fakeAnalysisService struct {
	last analysis.Request
	resp analysis.Response
	err  error
}

func (f *fakeAnalysisService) Run(_ context.Context, req analysis.Request) (analysis.Response, error) {
	f.last = req
	if f.err != nil {
		return analysis.Response{}, f.err
	}
	return f.resp, nil
}

func TestRunAnalysisUsesHeadersWhenBodyOmitsTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &fakeAnalysisService{
		resp: analysis.Response{
			Goal:    analysis.GoalAnalyzeIncident,
			Tenant:  "tenant-a",
			Service: "checkout",
			Health: analysis.HealthSummary{
				Status: "degraded",
			},
			Diagnosis: analysis.Diagnosis{
				Summary: "summary",
				Source:  "heuristic-fallback",
			},
			Meta: analysis.Meta{
				Reasoner:       "heuristic-fallback",
				ReasonerStatus: "fallback",
			},
		},
	}
	RegisterAnalysisRoutes(router, svc)

	body, _ := json.Marshal(map[string]interface{}{
		"goal":    "analyze_incident",
		"service": "checkout",
	})
	req := httptest.NewRequest(http.MethodPost, "/analysis/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant", "tenant-a")
	req.Header.Set("X-User", "operator-a")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if svc.last.Tenant != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", svc.last.Tenant)
	}
	if svc.last.User != "operator-a" {
		t.Fatalf("user = %q, want operator-a", svc.last.User)
	}
}
