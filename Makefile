# ============================================================
# LLM Client Go — Makefile
# ============================================================

MODULE      := llm-client-go
GOPATH      ?= $(shell go env GOPATH)
GOBIN       ?= $(GOPATH)/bin

# 빌드 결과물 디렉토리
BUILD_DIR   := bin

# 봇 바이너리 이름 → 소스 경로 매핑
BOTS        := discord_bot telegram_bot slack_bot
BOT_SRCS    := $(addprefix examples/, $(BOTS))

# 예제 바이너리 이름 → 소스 경로 매핑
EXAMPLES    := openai_chat openai_stream openai_tools azure_chat azure_stream
EXAMPLE_SRCS := $(addprefix examples/, $(EXAMPLES))

# 모든 바이너리
ALL_BINS    := $(BOTS) $(EXAMPLES)

# 빌드 플래그
LDFLAGS     := -s -w
GOFLAGS     := -trimpath

# 크로스컴파일 대상 플랫폼 (OS/ARCH)
PLATFORMS   := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.DEFAULT_GOAL := help

# ─── 도움말 ────────────────────────────────────────────────

.PHONY: help
help: ## 사용 가능한 명령어 목록 출력
	@echo ""
	@echo "  LLM Client Go — Makefile"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_\/-]+:.*##/ { \
		printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""

# ─── 의존성 ────────────────────────────────────────────────

.PHONY: deps
deps: ## 의존성 다운로드 (go mod download)
	go mod download

.PHONY: deps/tidy
deps/tidy: ## go.mod / go.sum 정리 (go mod tidy)
	go mod tidy

.PHONY: deps/update
deps/update: ## 모든 의존성을 최신 버전으로 업데이트
	go get -u ./...
	go mod tidy

# ─── 빌드 ──────────────────────────────────────────────────

.PHONY: build
build: build/bots build/examples ## 봇 + 예제 전체 빌드

.PHONY: build/bots
build/bots: $(BUILD_DIR) ## 봇 바이너리 빌드 (discord_bot, telegram_bot, slack_bot)
	@echo "▶ Building bots..."
	@for name in $(BOTS); do \
		echo "  → $$name"; \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$$name ./examples/$$name; \
	done

.PHONY: build/examples
build/examples: $(BUILD_DIR) ## 예제 바이너리 빌드
	@echo "▶ Building examples..."
	@for name in $(EXAMPLES); do \
		echo "  → $$name"; \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$$name ./examples/$$name; \
	done

.PHONY: build/all
build/all: build ## build 와 동일 (전체 빌드)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# ─── 크로스컴파일 ──────────────────────────────────────────

.PHONY: build/cross
build/cross: build/cross/bots build/cross/examples ## 전 플랫폼 크로스컴파일 (bots + examples)

.PHONY: build/cross/bots
build/cross/bots: ## 봇 바이너리 크로스컴파일 (bin/<os>_<arch>/)
	@echo "▶ Cross-compiling bots for: $(PLATFORMS)"
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d/ -f1); \
		arch=$$(echo $$platform | cut -d/ -f2); \
		outdir=$(BUILD_DIR)/$${os}_$${arch}; \
		mkdir -p $$outdir; \
		echo "  [$$os/$$arch]"; \
		for name in $(BOTS); do \
			out=$$outdir/$$name; \
			[ "$$os" = "windows" ] && out=$${out}.exe; \
			echo "    → $$name"; \
			GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $$out ./examples/$$name || exit 1; \
		done; \
	done

.PHONY: build/cross/examples
build/cross/examples: ## 예제 바이너리 크로스컴파일 (bin/<os>_<arch>/)
	@echo "▶ Cross-compiling examples for: $(PLATFORMS)"
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d/ -f1); \
		arch=$$(echo $$platform | cut -d/ -f2); \
		outdir=$(BUILD_DIR)/$${os}_$${arch}; \
		mkdir -p $$outdir; \
		echo "  [$$os/$$arch]"; \
		for name in $(EXAMPLES); do \
			out=$$outdir/$$name; \
			[ "$$os" = "windows" ] && out=$${out}.exe; \
			echo "    → $$name"; \
			GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $$out ./examples/$$name || exit 1; \
		done; \
	done

# ─── 실행 ──────────────────────────────────────────────────

.PHONY: run/discord
run/discord: ## Discord 봇 실행 (환경변수 필요: DISCORD_BOT_TOKEN, OPENAI_API_KEY)
	go run ./examples/discord_bot

.PHONY: run/telegram
run/telegram: ## Telegram 봇 실행 (환경변수 필요: TELEGRAM_BOT_TOKEN, OPENAI_API_KEY)
	go run ./examples/telegram_bot

.PHONY: run/slack
run/slack: ## Slack 봇 실행 (환경변수 필요: SLACK_BOT_TOKEN, SLACK_APP_TOKEN, OPENAI_API_KEY)
	go run ./examples/slack_bot

.PHONY: run/openai-chat
run/openai-chat: ## OpenAI 채팅 예제 실행
	go run ./examples/openai_chat

.PHONY: run/openai-stream
run/openai-stream: ## OpenAI 스트리밍 예제 실행
	go run ./examples/openai_stream

.PHONY: run/openai-tools
run/openai-tools: ## OpenAI Function Calling 예제 실행
	go run ./examples/openai_tools

.PHONY: run/azure-chat
run/azure-chat: ## Azure OpenAI 채팅 예제 실행
	go run ./examples/azure_chat

.PHONY: run/azure-stream
run/azure-stream: ## Azure OpenAI 스트리밍 예제 실행
	go run ./examples/azure_stream

# ─── 테스트 ────────────────────────────────────────────────

.PHONY: test
test: ## 전체 테스트 실행
	go test ./...

.PHONY: test/verbose
test/verbose: ## 전체 테스트 실행 (상세 출력)
	go test -v ./...

.PHONY: test/coverage
test/coverage: ## 커버리지 리포트 생성 (coverage.html)
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: test/race
test/race: ## 레이스 컨디션 감지 포함 테스트
	go test -race ./...

# ─── 코드 품질 ─────────────────────────────────────────────

.PHONY: lint
lint: ## go vet 정적 분석 실행
	go vet ./...

.PHONY: fmt
fmt: ## 코드 포매팅 (gofmt)
	gofmt -w -s .

.PHONY: fmt/check
fmt/check: ## 포매팅 검사만 수행 (수정 없음)
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "포매팅이 필요한 파일:"; echo "$$out"; exit 1; \
	else \
		echo "모든 파일이 올바르게 포매팅되어 있습니다."; \
	fi

# ─── 설치 / 제거 ───────────────────────────────────────────

.PHONY: install
install: build ## 봇 바이너리를 GOBIN($(GOBIN))에 설치
	@echo "▶ Installing bots to $(GOBIN)..."
	@for name in $(BOTS); do \
		echo "  → $$name"; \
		cp $(BUILD_DIR)/$$name $(GOBIN)/$$name; \
	done
	@echo "✅ 설치 완료. PATH에 $(GOBIN)이 포함되어 있는지 확인하세요."

.PHONY: uninstall
uninstall: ## GOBIN에서 봇 바이너리 제거
	@echo "▶ Uninstalling bots from $(GOBIN)..."
	@for name in $(BOTS); do \
		if [ -f "$(GOBIN)/$$name" ]; then \
			echo "  → removing $$name"; \
			rm -f $(GOBIN)/$$name; \
		fi \
	done
	@echo "✅ 제거 완료."

# ─── 정리 ──────────────────────────────────────────────────

.PHONY: clean
clean: ## 빌드 결과물(bin/) 및 커버리지 파일 삭제
	@echo "▶ Cleaning..."
	rm -rf $(BUILD_DIR) coverage.out coverage.html
	@echo "✅ 완료."

.PHONY: clean/cache
clean/cache: ## Go 빌드 캐시 삭제
	go clean -cache

.PHONY: clean/all
clean/all: clean clean/cache ## 빌드 결과물 + Go 캐시 전체 삭제

# ─── 정보 ──────────────────────────────────────────────────

.PHONY: info
info: ## 환경 정보 출력 (Go 버전, GOPATH, GOBIN 등)
	@echo "  Module  : $(MODULE)"
	@echo "  Go      : $$(go version)"
	@echo "  GOPATH  : $(GOPATH)"
	@echo "  GOBIN   : $(GOBIN)"
	@echo "  Build   : $(BUILD_DIR)/"
	@echo "  Bots    : $(BOTS)"
	@echo "  Examples: $(EXAMPLES)"

.PHONY: check-env/discord
check-env/discord: ## Discord 봇에 필요한 환경변수 확인
	@missing=""; \
	for v in DISCORD_BOT_TOKEN OPENAI_API_KEY; do \
		[ -z "$${!v}" ] && missing="$$missing $$v"; \
	done; \
	if [ -n "$$missing" ]; then \
		echo "❌ 누락된 환경변수:$$missing"; exit 1; \
	else echo "✅ Discord 환경변수 OK"; fi

.PHONY: check-env/telegram
check-env/telegram: ## Telegram 봇에 필요한 환경변수 확인
	@missing=""; \
	for v in TELEGRAM_BOT_TOKEN OPENAI_API_KEY; do \
		[ -z "$${!v}" ] && missing="$$missing $$v"; \
	done; \
	if [ -n "$$missing" ]; then \
		echo "❌ 누락된 환경변수:$$missing"; exit 1; \
	else echo "✅ Telegram 환경변수 OK"; fi

.PHONY: check-env/slack
check-env/slack: ## Slack 봇에 필요한 환경변수 확인
	@missing=""; \
	for v in SLACK_BOT_TOKEN SLACK_APP_TOKEN OPENAI_API_KEY; do \
		[ -z "$${!v}" ] && missing="$$missing $$v"; \
	done; \
	if [ -n "$$missing" ]; then \
		echo "❌ 누락된 환경변수:$$missing"; exit 1; \
	else echo "✅ Slack 환경변수 OK"; fi
