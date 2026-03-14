package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/xscopehub/mcp-server/internal/types"
)

const (
	defaultWindow = "1h"
)

type ObservabilityPluginConfig struct {
	ObserveGatewayURL string
	LlmOpsAgentURL    string
	DefaultTenant     string
	DefaultUser       string
	TenantHeader      string
	UserHeader        string
	Timeout           time.Duration
}

type ObservabilityPlugin struct {
	config     map[string]interface{}
	client     *http.Client
	gatewayURL string
	agentURL   string
	tenant     string
	user       string
	tenantHdr  string
	userHdr    string
}

func NewObservabilityPlugin(cfg ObservabilityPluginConfig) *ObservabilityPlugin {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	tenantHeader := strings.TrimSpace(cfg.TenantHeader)
	if tenantHeader == "" {
		tenantHeader = "X-Tenant"
	}
	userHeader := strings.TrimSpace(cfg.UserHeader)
	if userHeader == "" {
		userHeader = "X-User"
	}
	return &ObservabilityPlugin{
		client:     &http.Client{Timeout: timeout},
		gatewayURL: strings.TrimRight(cfg.ObserveGatewayURL, "/"),
		agentURL:   strings.TrimRight(cfg.LlmOpsAgentURL, "/"),
		tenant:     cfg.DefaultTenant,
		user:       cfg.DefaultUser,
		tenantHdr:  tenantHeader,
		userHdr:    userHeader,
	}
}

func (p *ObservabilityPlugin) ID() string   { return "observability" }
func (p *ObservabilityPlugin) Name() string { return "Observability Hub" }
func (p *ObservabilityPlugin) Description() string {
	return "Monitoring analysis tools backed by observe-gateway and llm-ops-agent"
}

func (p *ObservabilityPlugin) Init(config map[string]interface{}) error {
	p.config = config
	return nil
}

func (p *ObservabilityPlugin) Resources() []types.ResourcePayload {
	return []types.ResourcePayload{
		{
			Name:        "monitoring_capabilities",
			Description: "Available monitoring analysis surfaces exposed by XScopeHub.",
			Data: map[string]interface{}{
				"queries": []string{"query_metrics", "query_logs", "query_traces"},
				"analysis": []string{
					"get_topology",
					"analyze_incident",
					"summarize_service_health",
					"propose_remediation",
				},
			},
		},
		{
			Name:        "analysis_templates",
			Description: "Default observe-gateway templates used by the monitoring agent.",
			Data: map[string]interface{}{
				"metrics":  []string{"service_error_rate", "service_latency_p95"},
				"logs":     []string{"service_error_logs"},
				"traces":   []string{"service_error_traces"},
				"topology": []string{"service_topology_logs"},
			},
		},
	}
}

func (p *ObservabilityPlugin) Tools() []types.ToolDescriptor {
	return []types.ToolDescriptor{
		{
			Name:        "query_metrics",
			Description: "Query service metrics through observe-gateway templates.",
			InputSchema: `{"type":"object","required":["service"],"properties":{"service":{"type":"string"},"tenant":{"type":"string"},"user":{"type":"string"},"metric":{"type":"string","enum":["error_rate","latency_p95"]},"window":{"type":"string"},"start":{"type":"string","format":"date-time"},"end":{"type":"string","format":"date-time"}}}`,
		},
		{
			Name:        "query_logs",
			Description: "Query error logs for a service through observe-gateway.",
			InputSchema: `{"type":"object","required":["service"],"properties":{"service":{"type":"string"},"tenant":{"type":"string"},"user":{"type":"string"},"window":{"type":"string"},"start":{"type":"string","format":"date-time"},"end":{"type":"string","format":"date-time"}}}`,
		},
		{
			Name:        "query_traces",
			Description: "Query failing traces for a service through observe-gateway.",
			InputSchema: `{"type":"object","required":["service"],"properties":{"service":{"type":"string"},"tenant":{"type":"string"},"user":{"type":"string"},"window":{"type":"string"},"start":{"type":"string","format":"date-time"},"end":{"type":"string","format":"date-time"}}}`,
		},
		{
			Name:        "get_topology",
			Description: "Summarize service topology evidence using llm-ops-agent.",
			InputSchema: `{"type":"object","required":["service"],"properties":{"service":{"type":"string"},"tenant":{"type":"string"},"user":{"type":"string"},"window":{"type":"string"},"start":{"type":"string","format":"date-time"},"end":{"type":"string","format":"date-time"},"incident":{"type":"string"}}}`,
		},
		{
			Name:        "analyze_incident",
			Description: "Run full incident analysis against metrics, logs, traces, and topology.",
			InputSchema: `{"type":"object","required":["service"],"properties":{"service":{"type":"string"},"tenant":{"type":"string"},"user":{"type":"string"},"incident":{"type":"string"},"window":{"type":"string"},"start":{"type":"string","format":"date-time"},"end":{"type":"string","format":"date-time"},"prompt":{"type":"string"}}}`,
		},
		{
			Name:        "summarize_service_health",
			Description: "Produce a structured health summary for a service.",
			InputSchema: `{"type":"object","required":["service"],"properties":{"service":{"type":"string"},"tenant":{"type":"string"},"user":{"type":"string"},"window":{"type":"string"},"start":{"type":"string","format":"date-time"},"end":{"type":"string","format":"date-time"},"incident":{"type":"string"}}}`,
		},
		{
			Name:        "propose_remediation",
			Description: "Generate remediation guidance based on current evidence.",
			InputSchema: `{"type":"object","required":["service"],"properties":{"service":{"type":"string"},"tenant":{"type":"string"},"user":{"type":"string"},"window":{"type":"string"},"start":{"type":"string","format":"date-time"},"end":{"type":"string","format":"date-time"},"incident":{"type":"string"},"prompt":{"type":"string"}}}`,
		},
	}
}

func (p *ObservabilityPlugin) ExecuteTool(name string, args map[string]interface{}) (types.ToolResult, error) {
	switch name {
	case "query_metrics":
		return p.queryMetrics(args)
	case "query_logs":
		return p.queryTemplateTool(name, "service_error_logs", args)
	case "query_traces":
		return p.queryTemplateTool(name, "service_error_traces", args)
	case "get_topology":
		return p.runAnalysis("get_topology", args)
	case "analyze_incident":
		return p.runAnalysis("analyze_incident", args)
	case "summarize_service_health":
		return p.runAnalysis("summarize_service_health", args)
	case "propose_remediation":
		return p.runAnalysis("propose_remediation", args)
	default:
		return types.ToolResult{}, fmt.Errorf("tool %s not found in observability plugin", name)
	}
}

func (p *ObservabilityPlugin) queryMetrics(args map[string]interface{}) (types.ToolResult, error) {
	parsed, err := parseSharedArgs(args)
	if err != nil {
		return types.ToolResult{}, err
	}
	if parsed.Tenant == "" {
		parsed.Tenant = p.tenant
	}
	if parsed.User == "" {
		parsed.User = p.user
	}

	type metricQuery struct {
		Key      string
		Template string
	}
	queries := []metricQuery{}
	switch parsed.Metric {
	case "", "error_rate":
		queries = append(queries, metricQuery{Key: "error_rate", Template: "service_error_rate"})
	}
	if parsed.Metric == "" || parsed.Metric == "latency_p95" {
		queries = append(queries, metricQuery{Key: "latency_p95", Template: "service_latency_p95"})
	}

	results := make(map[string]interface{}, len(queries))
	for _, query := range queries {
		resp, err := p.queryObserveGateway(parsed, query.Template)
		if err != nil {
			return types.ToolResult{}, err
		}
		results[query.Key] = resp
	}

	return types.ToolResult{
		Name: "query_metrics",
		Output: map[string]interface{}{
			"tenant":  parsed.Tenant,
			"service": parsed.Service,
			"window":  parsed.Window,
			"results": results,
		},
	}, nil
}

func (p *ObservabilityPlugin) queryTemplateTool(name string, template string, args map[string]interface{}) (types.ToolResult, error) {
	parsed, err := parseSharedArgs(args)
	if err != nil {
		return types.ToolResult{}, err
	}
	if parsed.Tenant == "" {
		parsed.Tenant = p.tenant
	}
	if parsed.User == "" {
		parsed.User = p.user
	}
	resp, err := p.queryObserveGateway(parsed, template)
	if err != nil {
		return types.ToolResult{}, err
	}
	return types.ToolResult{
		Name: name,
		Output: map[string]interface{}{
			"tenant":   parsed.Tenant,
			"service":  parsed.Service,
			"window":   parsed.Window,
			"template": template,
			"result":   resp,
		},
	}, nil
}

func (p *ObservabilityPlugin) runAnalysis(goal string, args map[string]interface{}) (types.ToolResult, error) {
	if p.agentURL == "" {
		return types.ToolResult{}, fmt.Errorf("llm-ops-agent url is not configured")
	}

	parsed, err := parseSharedArgs(args)
	if err != nil {
		return types.ToolResult{}, err
	}
	if parsed.Tenant == "" {
		parsed.Tenant = p.tenant
	}
	if parsed.User == "" {
		parsed.User = p.user
	}

	payload := map[string]interface{}{
		"goal":     goal,
		"tenant":   parsed.Tenant,
		"user":     parsed.User,
		"service":  parsed.Service,
		"incident": parsed.Incident,
		"window":   parsed.Window,
		"start":    parsed.Start,
		"end":      parsed.End,
		"prompt":   parsed.Prompt,
	}

	response, err := p.postJSON(context.Background(), p.agentURL+"/analysis/run", payload, parsed.Tenant, parsed.User)
	if err != nil {
		return types.ToolResult{}, err
	}
	return types.ToolResult{Name: goal, Output: response}, nil
}

func (p *ObservabilityPlugin) queryObserveGateway(args sharedArgs, template string) (map[string]interface{}, error) {
	if p.gatewayURL == "" {
		return nil, fmt.Errorf("observe-gateway url is not configured")
	}
	payload := map[string]interface{}{
		"template": template,
		"variables": map[string]string{
			"service": args.Service,
			"window":  args.Window,
		},
		"start": args.Start,
		"end":   args.End,
	}
	return p.postJSON(context.Background(), p.gatewayURL+"/api/query", payload, args.Tenant, args.User)
}

func (p *ObservabilityPlugin) postJSON(ctx context.Context, url string, payload interface{}, tenant string, user string) (map[string]interface{}, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if tenant != "" {
		req.Header.Set(p.tenantHdr, tenant)
	}
	if user != "" {
		req.Header.Set(p.userHdr, user)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call upstream %s: %w", url, err)
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode upstream response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if msg, ok := body["error"].(string); ok && msg != "" {
			return nil, fmt.Errorf("upstream %s returned %d: %s", url, resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("upstream %s returned %d", url, resp.StatusCode)
	}
	return body, nil
}

type sharedArgs struct {
	Service  string                 `json:"service"`
	Tenant   string                 `json:"tenant"`
	User     string                 `json:"user"`
	Incident string                 `json:"incident"`
	Metric   string                 `json:"metric"`
	Window   string                 `json:"window"`
	Prompt   string                 `json:"prompt"`
	StartRaw string                 `json:"start"`
	EndRaw   string                 `json:"end"`
	Headers  map[string]interface{} `json:"_mcp_request_headers"`
	Start    time.Time              `json:"-"`
	End      time.Time              `json:"-"`
}

func parseSharedArgs(args map[string]interface{}) (sharedArgs, error) {
	var parsed sharedArgs
	raw, err := json.Marshal(args)
	if err != nil {
		return sharedArgs{}, fmt.Errorf("marshal args: %w", err)
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return sharedArgs{}, fmt.Errorf("decode args: %w", err)
	}
	if strings.TrimSpace(parsed.Service) == "" {
		return sharedArgs{}, fmt.Errorf("service is required")
	}
	if parsed.Window == "" {
		parsed.Window = defaultWindow
	}
	if parsed.Tenant == "" {
		parsed.Tenant = headerValue(parsed.Headers, "X-Tenant", "x-tenant", "X-Scope-Tenant")
	}
	if parsed.User == "" {
		parsed.User = headerValue(parsed.Headers, "X-User", "x-user", "X-Scope-User")
	}
	var errStart error
	if parsed.StartRaw != "" {
		parsed.Start, errStart = time.Parse(time.RFC3339, parsed.StartRaw)
		if errStart != nil {
			return sharedArgs{}, fmt.Errorf("invalid start: %w", errStart)
		}
	}
	var errEnd error
	if parsed.EndRaw != "" {
		parsed.End, errEnd = time.Parse(time.RFC3339, parsed.EndRaw)
		if errEnd != nil {
			return sharedArgs{}, fmt.Errorf("invalid end: %w", errEnd)
		}
	}
	if parsed.End.IsZero() {
		parsed.End = time.Now().UTC()
	}
	if parsed.Start.IsZero() {
		window, err := time.ParseDuration(parsed.Window)
		if err != nil {
			window = time.Hour
		}
		parsed.Start = parsed.End.Add(-window)
	}
	return parsed, nil
}

func headerValue(headers map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := headers[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
