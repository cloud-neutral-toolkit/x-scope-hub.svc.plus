.PHONY: build test clean run docker helm \
            build-llm build-obs build-obsgw build-mcp \
            test-llm test-obs test-obsgw test-mcp \
            clean-llm clean-obs clean-obsgw clean-mcp \
            run-obs run-obsgw run-mcp run-llm-ops-agent \
            docker-obs helm-obs integration-tests integration-tests-llm integration-tests-obs \
            codex-home codex-build codex-run render-openclaw-config register-openclaw smoke-monitor-stack

LLM_DIR := llm-ops-agent
OBS_DIR := observe-bridge
OBSGW_DIR := observe-gateway
MCP_DIR := mcp-server

build: build-llm build-obs build-obsgw build-mcp

test: test-llm test-obs test-obsgw test-mcp

clean: clean-llm clean-obs clean-obsgw clean-mcp

run: run-obsgw run-llm-ops-agent run-mcp

docker: docker-obs

helm: helm-obs

build-llm:
	$(MAKE) -C $(LLM_DIR) build

test-llm:
	$(MAKE) -C $(LLM_DIR) test

clean-llm:
	$(MAKE) -C $(LLM_DIR) clean

build-obs:
	$(MAKE) -C $(OBS_DIR) build

build-obsgw:
	cd $(OBSGW_DIR) && go build ./...

build-mcp:
	cd $(MCP_DIR) && go build ./...

test-obs:
	$(MAKE) -C $(OBS_DIR) test

test-obsgw:
	cd $(OBSGW_DIR) && go test ./...

test-mcp:
	cd $(MCP_DIR) && go test ./...

clean-obs:
	$(MAKE) -C $(OBS_DIR) clean

clean-obsgw:
	cd $(OBSGW_DIR) && go clean ./...

clean-mcp:
	cd $(MCP_DIR) && go clean ./...

run-obs:
	$(MAKE) -C $(OBS_DIR) run

run-obsgw:
	cd $(OBSGW_DIR) && go run ./cmd/gateway -config ../config/observe-gateway.yaml

run-llm-ops-agent:
	$(MAKE) -C $(LLM_DIR) run

run-mcp:
	cd $(MCP_DIR) && go run ./cmd/mcp serve -addr :$${XSCOPE_MCP_SERVER_PORT:-8000}

docker-obs:
	$(MAKE) -C $(OBS_DIR) docker

helm-obs:
	$(MAKE) -C $(OBS_DIR) helm

integration-tests: integration-tests-llm integration-tests-obs

integration-tests-llm:
	$(MAKE) -C $(LLM_DIR) integration-tests

integration-tests-obs:
	$(MAKE) -C $(OBS_DIR) integration-tests

codex-home:
	./scripts/codex/setup-project-home.sh

codex-build:
	./scripts/codex/build-local.sh

codex-run:
	./scripts/codex/run-monitor.sh

render-openclaw-config:
	./scripts/openclaw/render-xscope-monitor-config.sh

register-openclaw:
	./scripts/openclaw/register-x-observability-agent.sh

smoke-monitor-stack:
	./scripts/smoke-monitor-stack.sh
