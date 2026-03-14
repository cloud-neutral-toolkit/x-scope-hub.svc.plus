package analysis

import (
	"context"
	"encoding/json"
	"time"
)

type Goal string

const (
	GoalAnalyzeIncident        Goal = "analyze_incident"
	GoalSummarizeServiceHealth      = "summarize_service_health"
	GoalProposeRemediation          = "propose_remediation"
	GoalGetTopology                 = "get_topology"
)

type Request struct {
	Goal     Goal      `json:"goal"`
	Tenant   string    `json:"tenant"`
	User     string    `json:"user"`
	Service  string    `json:"service"`
	Incident string    `json:"incident"`
	Window   string    `json:"window"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	MaxItems int       `json:"max_items"`
	Prompt   string    `json:"prompt"`
}

type Response struct {
	Goal       Goal          `json:"goal"`
	Tenant     string        `json:"tenant"`
	User       string        `json:"user,omitempty"`
	Service    string        `json:"service"`
	Incident   string        `json:"incident,omitempty"`
	TimeWindow TimeWindow    `json:"time_window"`
	Health     HealthSummary `json:"health"`
	Evidence   EvidenceSet   `json:"evidence"`
	Diagnosis  Diagnosis     `json:"diagnosis"`
	Meta       Meta          `json:"meta"`
}

type TimeWindow struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	Window string    `json:"window"`
}

type HealthSummary struct {
	Status     string   `json:"status"`
	Highlights []string `json:"highlights"`
}

type EvidenceSet struct {
	Metrics  map[string]Evidence `json:"metrics"`
	Logs     Evidence            `json:"logs"`
	Traces   Evidence            `json:"traces"`
	Topology Evidence            `json:"topology"`
}

type Evidence struct {
	Template string          `json:"template"`
	Lang     string          `json:"lang"`
	Backend  string          `json:"backend,omitempty"`
	Cached   bool            `json:"cached"`
	Count    int             `json:"count"`
	Result   json.RawMessage `json:"result,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type Diagnosis struct {
	Summary      string   `json:"summary"`
	LikelyCauses []string `json:"likely_causes"`
	Evidence     []string `json:"evidence"`
	Remediation  []string `json:"remediation"`
	Confidence   float64  `json:"confidence"`
	Source       string   `json:"source"`
}

type Meta struct {
	Reasoner       string   `json:"reasoner"`
	ReasonerStatus string   `json:"reasoner_status"`
	Errors         []string `json:"errors,omitempty"`
}

type Service interface {
	Run(context.Context, Request) (Response, error)
}

type Reasoner interface {
	Analyze(context.Context, ReasonerInput) (Diagnosis, error)
}

type ReasonerInput struct {
	Request  Request
	Health   HealthSummary
	Evidence EvidenceSet
}
