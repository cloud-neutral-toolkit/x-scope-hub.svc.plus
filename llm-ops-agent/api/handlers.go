package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/yourname/XOpsAgent/internal/analysis"
	"github.com/yourname/XOpsAgent/internal/ports"
	"github.com/yourname/XOpsAgent/internal/services/orchestrator"
	"github.com/yourname/XOpsAgent/workflow"
)

// RegisterRoutes wires all HTTP handlers for the agent modules.
func RegisterRoutes(r gin.IRoutes, svc orchestrator.Service) {
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	RegisterA2ARoutes(r)

	if svc == nil {
		return
	}

	h := &caseHandler{svc: svc}
	r.POST("/case/create", h.createCase)
	r.PATCH("/case/:id/transition", h.transitionCase)
}

func RegisterAnalysisRoutes(r gin.IRoutes, svc analysis.Service) {
	if svc == nil {
		return
	}
	h := &analysisHandler{svc: svc}
	r.POST("/analysis/run", h.run)
}

type caseHandler struct {
	svc orchestrator.Service
}

type analysisHandler struct {
	svc analysis.Service
}

type createCaseReq struct {
	TenantID int64  `json:"tenant_id"`
	Title    string `json:"title"`
}

func (h *caseHandler) createCase(c *gin.Context) {
	var req createCaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	idem := c.GetHeader("Idempotency-Key")
	actor := c.GetHeader("X-Actor")
	row, err := h.svc.CreateCase(c.Request.Context(), ports.CreateCaseArgs{
		TenantID: req.TenantID,
		Title:    req.Title,
		Actor:    actor,
		IdemKey:  idem,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"case_id": row.CaseID.String(), "status": row.Status, "version": row.Version})
}

type transitionReq struct {
	Event string `json:"event"`
}

func (h *caseHandler) transitionCase(c *gin.Context) {
	var req transitionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	idStr := c.Param("id")
	uid, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var ver int64
	ifMatch := c.GetHeader("If-Match")
	if ifMatch != "" {
		ver, _ = strconv.ParseInt(ifMatch, 10, 64)
	}
	idem := c.GetHeader("Idempotency-Key")
	actor := c.GetHeader("X-Actor")
	ctx := workflow.Context{Now: time.Now(), Actor: actor}
	row, err := h.svc.Transition(c.Request.Context(), ports.TransitionArgs{
		CaseID:  pgtype.UUID{Bytes: uid, Valid: true},
		Event:   workflow.Event(req.Event),
		Ctx:     ctx,
		IfMatch: ver,
		IdemKey: idem,
		Request: []byte{},
	})
	if err != nil {
		if errors.Is(err, workflow.ErrIllegal) || errors.Is(err, workflow.ErrGuard) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusPreconditionFailed, gin.H{"error": "version mismatch"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("ETag", fmt.Sprintf("%d", row.Version))
	c.JSON(http.StatusOK, gin.H{"status": row.Status, "version": row.Version})
}

type runAnalysisReq struct {
	Goal     analysis.Goal `json:"goal"`
	Tenant   string        `json:"tenant"`
	User     string        `json:"user"`
	Service  string        `json:"service"`
	Incident string        `json:"incident"`
	Window   string        `json:"window"`
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	MaxItems int           `json:"max_items"`
	Prompt   string        `json:"prompt"`
}

func (h *analysisHandler) run(c *gin.Context) {
	var req runAnalysisReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Tenant == "" {
		req.Tenant = c.GetHeader("X-Tenant")
	}
	if req.User == "" {
		req.User = c.GetHeader("X-User")
	}

	resp, err := h.svc.Run(c.Request.Context(), analysis.Request{
		Goal:     req.Goal,
		Tenant:   req.Tenant,
		User:     req.User,
		Service:  req.Service,
		Incident: req.Incident,
		Window:   req.Window,
		Start:    req.Start,
		End:      req.End,
		MaxItems: req.MaxItems,
		Prompt:   req.Prompt,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
