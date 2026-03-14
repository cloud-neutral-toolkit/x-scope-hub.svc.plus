package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestA2AObservabilityNeedsAutomationInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterA2ARoutes(r)

	body := `{"from_agent_id":"x-automation-agent","request_id":"req-1","intent":"validate","goal":"validate deploy metrics after terraform apply"}`
	req := httptest.NewRequest(http.MethodPost, "/a2a/v1/negotiate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "needs_input" {
		t.Fatalf("expected needs_input, got %#v", resp["status"])
	}
}

func TestA2ATaskCreateAndFetch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterA2ARoutes(r)

	body := `{"from_agent_id":"xops-agent","request_id":"req-2","intent":"collect","goal":"collect logs and traces for checkout alert"}`
	req := httptest.NewRequest(http.MethodPost, "/a2a/v1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	taskID, _ := created["task_id"].(string)
	if taskID == "" {
		t.Fatalf("expected task id")
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/a2a/v1/tasks/"+taskID, nil)
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wGet.Code)
	}
}
