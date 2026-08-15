# LLM Client Go

[![Go Reference](https://pkg.go.dev/badge/github.com/wkqco33/LLM_client_go.svg)](https://pkg.go.dev/github.com/wkqco33/LLM_client_go)
[![CI](https://github.com/wkqco33/LLM_client_go/actions/workflows/ci.yml/badge.svg)](https://github.com/wkqco33/LLM_client_go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/wkqco33/LLM_client_go)](https://goreportcard.com/report/github.com/wkqco33/LLM_client_go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Go로 구현된 통합 LLM 클라이언트 및 에이전트 프레임워크입니다.  
OpenAI, Azure OpenAI, Ollama(로컬)를 하나의 인터페이스로 다루며, **MCP(Model Context Protocol)** 연동과 자동 도구 실행 에이전트를 지원합니다.

[English README](README_EN.md)

## 설치

```bash
go get github.com/wkqco33/LLM_client_go
```

## 핵심 특징

| 기능 | 설명 |
| - | - |
| **통합 인터페이스** | `llm.Client` 하나로 OpenAI, Azure, Ollama(로컬) 모델 제어 |
| **자동화 에이전트** | `agent.Runner`를 통한 도구(Function) 호출 루프 자동화 |
| **MCP 연동** | HTTP 및 Stdio(JSON-RPC) 전송 방식을 통한 MCP 서버 도구 연결 |
| **안정성 (Retry)** | 지수 백오프 및 Jitter가 적용된 자동 재시도 미들웨어 내장 |
| **RAG 지원** | Embeddings API 통합 인터페이스 제공 |
| **유틸리티** | 모델별 최적화된 토큰 계산기 및 컨텍스트 관리 |
| **봇 어댑터** | Discord, Telegram, Slack 봇 즉시 연결 |

---

## 패키지 구조

```bash
LLM_client_go/
├── types.go                  # 공통 타입 및 llm.Client 인터페이스
├── agent/                    # 에이전트 자동화 루프 (Runner)
├── mcp/                      # MCP 클라이언트 및 에이전트 브릿지
├── retry/                    # 자동 재시도 미들웨어 (RoundTripper)
├── token/                    # 토큰 계산기 유틸리티
├── openai/                   # OpenAI 프로바이더 구현
├── azure/                    # Azure OpenAI 프로바이더 구현
├── ollama/                   # Ollama 프로바이더 구현 (OpenAI 호환, 로컬 실행)
├── bots/                     # 메신저 봇 어댑터 (Discord, Telegram, Slack)
└── examples/                 # 다양한 사용 사례 예제 코드
```

---

## 1. 통합 클라이언트 사용법

어떤 프로바이더든 `llm.Client` 인터페이스를 구현하므로 동일한 방식으로 요청을 보낼 수 있습니다.

```go
// OpenAI, Azure, Ollama(로컬) 중 선택 — 재시도는 기본 적용되므로 별도 설정 불필요
var client llm.Client = openai.New(openai.Config{
    APIKey: os.Getenv("OPENAI_API_KEY"),
})

resp, err := client.Complete(ctx, llm.ChatRequest{
    Model: "gpt-4o",
    Messages: []llm.Message{
        {Role: llm.RoleUser, Content: "Hello!"},
    },
})
```

---

## 2. 에이전트 및 도구 자동화

`agent.Runner`를 사용하면 모델이 도구 호출을 요청했을 때 로컬 함수를 실행하고 결과를 다시 전달하는 루프를 자동으로 수행합니다.
한 턴에 여러 도구를 동시에 요청하면 병렬로 실행되며, 결과는 원래 요청 순서대로 대화 기록에 반영됩니다.

```go
runner := agent.NewRunner(client, "gpt-4o", 
    agent.WithSystemPrompt("You are a helpful assistant."),
    agent.WithMaxHistory(10), // 대화 이력 자동 관리
)

// 실행 가능한 도구 등록
runner.RegisterTool(&weatherTool{}) 

// 최종 응답이 나올 때까지 내부적으로 여러 번 상호작용
msgs, resp, err := runner.Run(ctx, messages)
```

---

## 3. MCP (Model Context Protocol) 연동

외부 MCP 서버(로컬 프로세스 또는 원격 서버)에서 제공하는 도구들을 에이전트에 동적으로 주입할 수 있습니다.

```go
// 1. MCP 서버 연결 (Stdio 방식 예시)
mcpProvider, _ := mcp.NewStdioClient("npx", "-y", "@modelcontextprotocol/server-filesystem", "/home/docs")

// 2. MCP 도구들을 에이전트용 실행 도구로 변환하여 등록
tools, _ := mcpProvider.ListTools(ctx)
for _, t := range mcp.WrapTools(mcpProvider, tools) {
    runner.RegisterTool(t)
}

// 3. 에이전트 실행 (이제 LLM이 파일 시스템 도구를 사용함)
runner.Run(ctx, messages)
```

---

## 4. 고급 기능

### 자동 재시도 (Retry)

네트워크 오류나 429(Rate Limit)/5xx 발생 시 서버의 `Retry-After` 헤더를 준수하며 지수 백오프를 수행합니다.
`RetryPolicy`를 따로 지정하지 않아도 `retry.DefaultPolicy`(최대 3회, 지수 백오프)가 모든 클라이언트에
기본 적용됩니다.

```go
// 정책 커스터마이즈
policy := retry.Policy{
    MaxRetries: 5,
    MinWait:    2 * time.Second,
}
client := openai.New(openai.Config{
    APIKey:      "...",
    RetryPolicy: &policy,
})

// 재시도 비활성화
client := openai.New(openai.Config{
    APIKey:      "...",
    RetryPolicy: &retry.Policy{}, // MaxRetries: 0
})
```

### 토큰 계산기

모델에 전송하기 전에 토큰 수를 미리 예측하여 비용과 컨텍스트를 제어할 수 있습니다.

```go
count := token.Estimate("Hello world")
// 또는 메시지 리스트 전체 계산
msgTokens := token.DefaultCounter.CountMessages(history)
```

---

## 에러 처리

라이브러리는 HTTP 상태 코드에 따른 구조화된 센티넬 에러를 제공합니다.

| 에러 | 설명 |
| - | - |
| `ErrUnauthorized` | API 키 오류 |
| `ErrRateLimited` | 요청 한도 초과 (429) |
| `ErrBadRequest` | 잘못된 파라미터 요청 (400) |
| `ErrServerError` | 모델 프로바이더 서버 오류 (5xx) |
| `ErrStreamClosed` | 닫힌 스트림에 대한 작업 시도 |

---

## 빌드 및 실행 (Task)

빌드/테스트/예제 실행은 [Task](https://taskfile.dev)(`Taskfile.yml`)로 관리합니다.

```bash
task --list-all       # 사용 가능한 태스크 전체 목록
task build             # 봇 + 예제 바이너리 빌드 (bin/)
task build:cross       # 전 플랫폼 크로스컴파일
task test               # 전체 테스트 실행
task lint               # go vet
task fmt                # gofmt

task run:ollama-chat   # Ollama 채팅 예제 실행
task run:discord        # Discord 봇 실행 (기본 백엔드: Ollama, BACKEND=openai|azure로 전환)

task --watch test       # 파일 변경 감지 시 자동 재실행 (TDD 루프)
```

이 프로젝트는 **TDD**로 개발합니다 — 새 코드를 작성하기 전에 실패하는 테스트부터 작성하세요.
워크플로우와 테스트 관례는 [AGENTS.md](AGENTS.md)를 참고하세요.

---

## 예제 코드

상세한 사용법은 `examples/` 디렉토리를 참고하세요.

- [에이전트 및 MCP 연동](examples/mcp_agent/main.go)
- [Ollama(로컬) 사용](examples/ollama_chat/main.go)
- [OpenAI 도구 사용](examples/openai_tools/main.go)
- [메신저 봇 설정](docs/messenger-setup.md)

---

## 라이선스

이 프로젝트는 [MIT License](LICENSE)를 따릅니다.
