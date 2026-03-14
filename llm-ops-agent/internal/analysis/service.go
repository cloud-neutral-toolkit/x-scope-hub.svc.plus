package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	templateErrorRate   = "service_error_rate"
	templateLatencyP95  = "service_latency_p95"
	templateErrorLogs   = "service_error_logs"
	templateErrorTraces = "service_error_traces"
	templateTopology    = "service_topology_logs"
)

type Options struct {
	Gateway       GatewayOptions
	Reasoner      Reasoner
	DefaultTenant string
	DefaultUser   string
	DefaultWindow time.Duration
	MaxItems      int
}

type GatewayOptions struct {
	Endpoint     string
	Headers      map[string]string
	TenantHeader string
	UserHeader   string
	Timeout      time.Duration
}

type gatewayClient struct {
	endpoint     string
	headers      map[string]string
	tenantHeader string
	userHeader   string
	httpClient   *http.Client
}

type service struct {
	gateway       *gatewayClient
	reasoner      Reasoner
	defaultTenant string
	defaultUser   string
	defaultWindow time.Duration
	maxItems      int
}

type queryRequest struct {
	Lang      string            `json:"lang,omitempty"`
	Query     string            `json:"query,omitempty"`
	Template  string            `json:"template,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Start     time.Time         `json:"start,omitempty"`
	End       time.Time         `json:"end,omitempty"`
	Step      string            `json:"step,omitempty"`
	Normalize bool              `json:"normalize,omitempty"`
}

type queryResponse struct {
	Lang   string          `json:"lang"`
	Tenant string          `json:"tenant"`
	Result json.RawMessage `json:"result"`
	Stats  struct {
		Backend    string `json:"backend"`
		Cached     bool   `json:"cached"`
		DurationMS int64  `json:"duration_ms"`
		Cost       int64  `json:"cost"`
	} `json:"stats"`
}

type CodexReasonerConfig struct {
	Command    string
	Args       []string
	WorkingDir string
	Timeout    time.Duration
	Env        map[string]string
}

type codexReasoner struct {
	command    string
	args       []string
	workingDir string
	timeout    time.Duration
	env        map[string]string
}

type codexDiagnosis struct {
	Summary      string   `json:"summary"`
	LikelyCauses []string `json:"likely_causes"`
	Evidence     []string `json:"evidence"`
	Remediation  []string `json:"remediation"`
	Confidence   float64  `json:"confidence"`
}

func NewService(opts Options) (Service, error) {
	gateway, err := newGatewayClient(opts.Gateway)
	if err != nil {
		return nil, err
	}
	window := opts.DefaultWindow
	if window <= 0 {
		window = time.Hour
	}
	maxItems := opts.MaxItems
	if maxItems <= 0 {
		maxItems = 50
	}
	return &service{
		gateway:       gateway,
		reasoner:      opts.Reasoner,
		defaultTenant: opts.DefaultTenant,
		defaultUser:   opts.DefaultUser,
		defaultWindow: window,
		maxItems:      maxItems,
	}, nil
}

func NewCodexReasoner(cfg CodexReasonerConfig) Reasoner {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		command = filepath.Clean("./scripts/codex/run-monitor.sh")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &codexReasoner{
		command:    command,
		args:       append([]string{}, cfg.Args...),
		workingDir: cfg.WorkingDir,
		timeout:    timeout,
		env:        cloneStringMap(cfg.Env),
	}
}

func newGatewayClient(opts GatewayOptions) (*gatewayClient, error) {
	endpoint := strings.TrimSpace(opts.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("observe_gateway endpoint is required")
	}
	if !strings.HasSuffix(endpoint, "/api/query") {
		endpoint = strings.TrimRight(endpoint, "/") + "/api/query"
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	tenantHeader := strings.TrimSpace(opts.TenantHeader)
	if tenantHeader == "" {
		tenantHeader = "X-Tenant"
	}
	userHeader := strings.TrimSpace(opts.UserHeader)
	if userHeader == "" {
		userHeader = "X-User"
	}
	return &gatewayClient{
		endpoint:     endpoint,
		headers:      cloneStringMap(opts.Headers),
		tenantHeader: tenantHeader,
		userHeader:   userHeader,
		httpClient:   &http.Client{Timeout: timeout},
	}, nil
}

func (s *service) Run(ctx context.Context, req Request) (Response, error) {
	req = s.normalizeRequest(req)
	if strings.TrimSpace(req.Service) == "" {
		return Response{}, fmt.Errorf("service is required")
	}

	metrics, logs, traces, topology, errs := s.collectEvidence(ctx, req)
	health := summarizeHealth(metrics, logs, traces, topology, errs)

	diagnosis, status := heuristicDiagnosis(req, health, EvidenceSet{
		Metrics:  metrics,
		Logs:     logs,
		Traces:   traces,
		Topology: topology,
	}, errs), "fallback"
	if s.reasoner != nil {
		modelDiagnosis, err := s.reasoner.Analyze(ctx, ReasonerInput{
			Request: req,
			Health:  health,
			Evidence: EvidenceSet{
				Metrics:  metrics,
				Logs:     logs,
				Traces:   traces,
				Topology: topology,
			},
		})
		if err == nil {
			diagnosis = modelDiagnosis
			status = "completed"
		} else {
			errs = append(errs, fmt.Sprintf("reasoner: %v", err))
		}
	}

	return Response{
		Goal:     req.Goal,
		Tenant:   req.Tenant,
		User:     req.User,
		Service:  req.Service,
		Incident: req.Incident,
		TimeWindow: TimeWindow{
			Start:  req.Start,
			End:    req.End,
			Window: req.Window,
		},
		Health: health,
		Evidence: EvidenceSet{
			Metrics:  metrics,
			Logs:     logs,
			Traces:   traces,
			Topology: topology,
		},
		Diagnosis: diagnosis,
		Meta: Meta{
			Reasoner:       diagnosis.Source,
			ReasonerStatus: status,
			Errors:         errs,
		},
	}, nil
}

func (s *service) normalizeRequest(req Request) Request {
	if req.Goal == "" {
		req.Goal = GoalAnalyzeIncident
	}
	if strings.TrimSpace(req.Tenant) == "" {
		req.Tenant = s.defaultTenant
	}
	if strings.TrimSpace(req.User) == "" {
		req.User = s.defaultUser
	}
	if req.End.IsZero() {
		req.End = time.Now().UTC()
	}
	if req.Start.IsZero() {
		window := strings.TrimSpace(req.Window)
		if window == "" {
			req.Start = req.End.Add(-s.defaultWindow)
			req.Window = s.defaultWindow.String()
		} else if d, err := time.ParseDuration(window); err == nil {
			req.Start = req.End.Add(-d)
		} else {
			req.Start = req.End.Add(-s.defaultWindow)
			req.Window = s.defaultWindow.String()
		}
	}
	if strings.TrimSpace(req.Window) == "" {
		req.Window = req.End.Sub(req.Start).Round(time.Minute).String()
	}
	if req.MaxItems <= 0 {
		req.MaxItems = s.maxItems
	}
	return req
}

func (s *service) collectEvidence(ctx context.Context, req Request) (map[string]Evidence, Evidence, Evidence, Evidence, []string) {
	metrics := map[string]Evidence{}
	logs := Evidence{Template: templateErrorLogs, Lang: "logql"}
	traces := Evidence{Template: templateErrorTraces, Lang: "traceql"}
	topology := Evidence{Template: templateTopology, Lang: "logql"}

	type result struct {
		key      string
		evidence Evidence
		err      error
	}

	results := make(chan result, 5)
	var wg sync.WaitGroup
	run := func(key string, template string, lang string) {
		defer wg.Done()
		evidence, err := s.queryTemplate(ctx, req, template, lang)
		results <- result{key: key, evidence: evidence, err: err}
	}

	wg.Add(5)
	go run("metric_error_rate", templateErrorRate, "promql")
	go run("metric_latency_p95", templateLatencyP95, "promql")
	go run("logs", templateErrorLogs, "logql")
	go run("traces", templateErrorTraces, "traceql")
	go run("topology", templateTopology, "logql")

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []string
	for res := range results {
		if res.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", res.key, res.err))
			res.evidence.Error = res.err.Error()
		}
		switch res.key {
		case "metric_error_rate":
			metrics["error_rate"] = res.evidence
		case "metric_latency_p95":
			metrics["latency_p95"] = res.evidence
		case "logs":
			logs = res.evidence
		case "traces":
			traces = res.evidence
		case "topology":
			topology = res.evidence
		}
	}

	return metrics, logs, traces, topology, errs
}

func (s *service) queryTemplate(ctx context.Context, req Request, template string, lang string) (Evidence, error) {
	payload := queryRequest{
		Template: template,
		Variables: map[string]string{
			"service": req.Service,
			"window":  req.Window,
		},
		Start: req.Start,
		End:   req.End,
	}
	resp, err := s.gateway.Query(ctx, req.Tenant, req.User, payload)
	if err != nil {
		return Evidence{Template: template, Lang: lang}, err
	}
	return Evidence{
		Template: template,
		Lang:     resp.Lang,
		Backend:  resp.Stats.Backend,
		Cached:   resp.Stats.Cached,
		Count:    estimateCount(resp.Result),
		Result:   resp.Result,
	}, nil
}

func (c *gatewayClient) Query(ctx context.Context, tenant string, user string, payload queryRequest) (queryResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return queryResponse{}, fmt.Errorf("marshal gateway query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return queryResponse{}, fmt.Errorf("build gateway request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}
	if tenant != "" {
		req.Header.Set(c.tenantHeader, tenant)
	}
	if user != "" {
		req.Header.Set(c.userHeader, user)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return queryResponse{}, fmt.Errorf("call observe-gateway: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return queryResponse{}, fmt.Errorf("read observe-gateway response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var errPayload struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(responseBody, &errPayload); err == nil && strings.TrimSpace(errPayload.Error) != "" {
			return queryResponse{}, fmt.Errorf("observe-gateway: %s", errPayload.Error)
		}
		return queryResponse{}, fmt.Errorf("observe-gateway returned %d", resp.StatusCode)
	}

	var out queryResponse
	if err := json.Unmarshal(responseBody, &out); err != nil {
		return queryResponse{}, fmt.Errorf("decode observe-gateway response: %w", err)
	}
	return out, nil
}

func (r *codexReasoner) Analyze(ctx context.Context, input ReasonerInput) (Diagnosis, error) {
	prompt := buildCodexPrompt(input)
	timeout := r.timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append([]string{}, r.args...)
	args = append(args, prompt)
	cmd := exec.CommandContext(runCtx, r.command, args...)
	if r.workingDir != "" {
		cmd.Dir = r.workingDir
	}
	cmd.Env = os.Environ()
	for key, value := range r.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return Diagnosis{}, fmt.Errorf("%w: %s", err, trimmed)
		}
		return Diagnosis{}, err
	}

	payload, err := extractJSONObject(output)
	if err != nil {
		return Diagnosis{}, err
	}

	var parsed codexDiagnosis
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return Diagnosis{}, fmt.Errorf("parse codex response: %w", err)
	}
	if strings.TrimSpace(parsed.Summary) == "" {
		return Diagnosis{}, errors.New("codex response missing summary")
	}

	return Diagnosis{
		Summary:      parsed.Summary,
		LikelyCauses: parsed.LikelyCauses,
		Evidence:     parsed.Evidence,
		Remediation:  parsed.Remediation,
		Confidence:   parsed.Confidence,
		Source:       "codex",
	}, nil
}

func summarizeHealth(metrics map[string]Evidence, logs Evidence, traces Evidence, topology Evidence, errs []string) HealthSummary {
	highlights := make([]string, 0, 6)
	if evidence, ok := metrics["error_rate"]; ok && evidence.Error == "" {
		highlights = append(highlights, fmt.Sprintf("error-rate series returned %d result set(s)", evidence.Count))
	}
	if evidence, ok := metrics["latency_p95"]; ok && evidence.Error == "" {
		highlights = append(highlights, fmt.Sprintf("latency p95 series returned %d result set(s)", evidence.Count))
	}
	if logs.Error == "" {
		highlights = append(highlights, fmt.Sprintf("log evidence captured %d record(s)", logs.Count))
	}
	if traces.Error == "" {
		highlights = append(highlights, fmt.Sprintf("trace evidence captured %d record(s)", traces.Count))
	}
	if topology.Error == "" && topology.Count > 0 {
		highlights = append(highlights, fmt.Sprintf("topology evidence captured %d record(s)", topology.Count))
	}
	if len(errs) > 0 {
		highlights = append(highlights, fmt.Sprintf("%d evidence source(s) returned errors", len(errs)))
	}

	status := "healthy"
	if logs.Count > 0 || traces.Count > 0 || len(errs) > 0 {
		status = "degraded"
	}
	if logs.Count >= 25 || traces.Count >= 10 {
		status = "critical"
	}
	if len(highlights) == 0 {
		highlights = append(highlights, "no evidence returned from upstream sources")
	}
	return HealthSummary{Status: status, Highlights: highlights}
}

func heuristicDiagnosis(req Request, health HealthSummary, evidence EvidenceSet, errs []string) Diagnosis {
	summary := fmt.Sprintf("Service %s is %s in the selected time window.", req.Service, health.Status)
	if req.Incident != "" {
		summary = fmt.Sprintf("%s Incident context: %s.", summary, req.Incident)
	}

	likelyCauses := []string{
		"Investigate recent deployments, config changes, or dependency instability affecting the service.",
	}
	if evidence.Logs.Count > 0 {
		likelyCauses = append(likelyCauses, "Application errors are visible in logs and should be clustered by exception or upstream dependency.")
	}
	if evidence.Traces.Count > 0 {
		likelyCauses = append(likelyCauses, "Failing traces suggest request-path regressions or downstream latency amplification.")
	}

	remediation := []string{
		"Compare the impacted window against the previous healthy period using the same templates.",
		"Correlate failing requests with the latest deploy, feature flag, or infrastructure change.",
	}
	if req.Goal == GoalProposeRemediation {
		remediation = append(remediation, "Prepare a rollback or traffic-shift plan before applying risky changes in production.")
	}
	if req.Goal == GoalGetTopology {
		remediation = []string{"Inspect topology evidence to identify the dominant downstream dependencies and retry/error hotspots."}
	}

	evidenceLines := append([]string{}, health.Highlights...)
	if len(errs) > 0 {
		evidenceLines = append(evidenceLines, "Reasoner fallback was used because Codex did not produce a valid response.")
	}

	return Diagnosis{
		Summary:      summary,
		LikelyCauses: likelyCauses,
		Evidence:     evidenceLines,
		Remediation:  remediation,
		Confidence:   0.42,
		Source:       "heuristic-fallback",
	}
}

func buildCodexPrompt(input ReasonerInput) string {
	req := input.Request
	return fmt.Sprintf(`You are the monitoring analysis agent for XScopeHub.
Return JSON only with keys: summary, likely_causes, evidence, remediation, confidence.

Goal: %s
Service: %s
Tenant: %s
Incident: %s
Time window: %s to %s (%s)
Health status: %s
Health highlights:
- %s

Metrics error_rate:
%s

Metrics latency_p95:
%s

Logs:
%s

Traces:
%s

Topology:
%s

Additional operator prompt:
%s
`,
		req.Goal,
		req.Service,
		req.Tenant,
		req.Incident,
		req.Start.Format(time.RFC3339),
		req.End.Format(time.RFC3339),
		req.Window,
		input.Health.Status,
		strings.Join(input.Health.Highlights, "\n- "),
		truncateJSON(input.Evidence.Metrics["error_rate"].Result, 3000),
		truncateJSON(input.Evidence.Metrics["latency_p95"].Result, 3000),
		truncateJSON(input.Evidence.Logs.Result, 3000),
		truncateJSON(input.Evidence.Traces.Result, 3000),
		truncateJSON(input.Evidence.Topology.Result, 3000),
		req.Prompt,
	)
}

func truncateJSON(raw json.RawMessage, limit int) string {
	if len(raw) == 0 {
		return "null"
	}
	if len(raw) <= limit {
		return string(raw)
	}
	return string(raw[:limit]) + "...(truncated)"
}

func estimateCount(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	return countAny(payload)
}

func countAny(value any) int {
	switch typed := value.(type) {
	case []any:
		return len(typed)
	case map[string]any:
		for _, key := range []string{"hits", "items", "rows", "records", "series", "result", "data"} {
			child, ok := typed[key]
			if !ok {
				continue
			}
			if n := countAny(child); n > 0 {
				return n
			}
		}
		for _, child := range typed {
			if n := countAny(child); n > 0 {
				return n
			}
		}
	}
	return 0
}

func extractJSONObject(raw []byte) ([]byte, error) {
	text := bytes.TrimSpace(raw)
	start := bytes.IndexByte(text, '{')
	end := bytes.LastIndexByte(text, '}')
	if start == -1 || end == -1 || end < start {
		return nil, errors.New("no json object found in codex output")
	}
	return text[start : end+1], nil
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
