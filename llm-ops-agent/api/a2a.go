package api

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type a2aRequest struct {
	FromAgentID string         `json:"from_agent_id"`
	ToAgentID   string         `json:"to_agent_id"`
	RequestID   string         `json:"request_id"`
	Intent      string         `json:"intent"`
	Goal        string         `json:"goal"`
	Context     map[string]any `json:"context,omitempty"`
	Artifacts   map[string]any `json:"artifacts,omitempty"`
	Constraints []string       `json:"constraints,omitempty"`
}

type a2aResponse struct {
	Status         string         `json:"status"`
	OwnerAgentID   string         `json:"owner_agent_id"`
	Summary        string         `json:"summary"`
	RequiredInputs []string       `json:"required_inputs,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
	TaskID         string         `json:"task_id,omitempty"`
}

type a2aTaskRecord struct {
	TaskID       string         `json:"task_id"`
	RequestID    string         `json:"request_id"`
	FromAgentID  string         `json:"from_agent_id"`
	ToAgentID    string         `json:"to_agent_id"`
	Intent       string         `json:"intent"`
	Goal         string         `json:"goal"`
	Status       string         `json:"status"`
	OwnerAgentID string         `json:"owner_agent_id"`
	Summary      string         `json:"summary"`
	Result       map[string]any `json:"result,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type a2aStore struct {
	mu    sync.RWMutex
	tasks map[string]a2aTaskRecord
}

func newA2AStore() *a2aStore {
	return &a2aStore{tasks: make(map[string]a2aTaskRecord)}
}

func RegisterA2ARoutes(r gin.IRoutes) {
	store := newA2AStore()
	r.POST("/a2a/v1/negotiate", store.negotiate)
	r.POST("/a2a/v1/tasks", store.createTask)
	r.GET("/a2a/v1/tasks/:id", store.getTask)
}

func (s *a2aStore) negotiate(c *gin.Context) {
	req, ok := bindA2ARequest(c)
	if !ok {
		return
	}
	resp := evaluateObservabilityRequest(req)
	c.JSON(http.StatusOK, resp)
}

func (s *a2aStore) createTask(c *gin.Context) {
	req, ok := bindA2ARequest(c)
	if !ok {
		return
	}
	resp := evaluateObservabilityRequest(req)
	taskID := "a2a-" + requestSeed()
	record := a2aTaskRecord{
		TaskID:       taskID,
		RequestID:    req.RequestID,
		FromAgentID:  req.FromAgentID,
		ToAgentID:    fallback(req.ToAgentID, "x-observability-agent"),
		Intent:       req.Intent,
		Goal:         req.Goal,
		Status:       resp.Status,
		OwnerAgentID: "x-observability-agent",
		Summary:      resp.Summary,
		Result:       resp.Result,
		CreatedAt:    time.Now().UTC(),
	}
	if record.Status == "accepted" {
		record.Status = "completed"
		record.Summary = "x-observability-agent completed the observability evidence handoff."
		record.Result["deliverable"] = "observability_evidence"
	}
	s.mu.Lock()
	s.tasks[taskID] = record
	s.mu.Unlock()
	log.Printf("a2a task request_id=%s task_id=%s from=%s to=x-observability-agent status=%s", req.RequestID, taskID, req.FromAgentID, record.Status)
	c.JSON(http.StatusAccepted, record)
}

func (s *a2aStore) getTask(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("id"))
	s.mu.RLock()
	record, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, record)
}

func bindA2ARequest(c *gin.Context) (a2aRequest, bool) {
	var req a2aRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return a2aRequest{}, false
	}
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.RequestID == "" {
		req.RequestID = requestSeed()
	}
	return req, true
}

func evaluateObservabilityRequest(req a2aRequest) a2aResponse {
	text := strings.ToLower(strings.Join([]string{req.Intent, req.Goal}, " "))
	status := "accepted"
	summary := "x-observability-agent accepts the observability evidence request."
	result := map[string]any{
		"role":         "observability",
		"decision":     "accepted",
		"request_id":   req.RequestID,
		"target_agent": "x-observability-agent",
		"next_action":  "collect logs, metrics, traces, and alert context",
	}

	if containsAny(text, []string{"terraform", "pulumi", "dns", "playbook", "iac", "deploy"}) {
		status = "needs_input"
		summary = "x-observability-agent can validate the change, but the infrastructure action belongs to x-automation-agent."
		result["decision"] = "consult"
		result["handoff_agent_id"] = "x-automation-agent"
	}
	if containsAny(text, []string{"incident commander", "runbook", "execute", "remediate", "root cause"}) {
		status = "declined"
		summary = "x-observability-agent declines operational command and recommends xops-agent."
		result["decision"] = "handoff"
		result["handoff_agent_id"] = "xops-agent"
	}

	log.Printf("a2a negotiate request_id=%s from=%s to=x-observability-agent status=%s", req.RequestID, req.FromAgentID, status)
	return a2aResponse{
		Status:       status,
		OwnerAgentID: "x-observability-agent",
		Summary:      summary,
		Result:       result,
	}
}

func containsAny(text string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(text, candidate) {
			return true
		}
	}
	return false
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return def
}

func requestSeed() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	}
	return hex.EncodeToString(buf[:])
}
