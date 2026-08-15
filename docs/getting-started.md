# Getting Started — 다른 프로젝트에서 사용하기

이 문서는 `llm-client-go` 라이브러리를 외부 Go 프로젝트에서 가져다 사용하는 방법과 주요 고급 기능(Agent, MCP, Retry 등)의 사용법을 설명합니다.

---

## 1. 모듈 의존성 추가

Go 모듈 명령어로 최신 버전을 설치합니다.

```bash
go get github.com/wkqco33/LLM_client_go
```

`go.mod`에 자동으로 의존성이 추가됩니다:

```go
module my-project

go 1.25

require github.com/wkqco33/LLM_client_go v0.1.0
```

이후 의존성을 동기화합니다.

```bash
go mod tidy
```

---

## 2. 통합 클라이언트 사용하기

모든 프로바이더(OpenAI, Azure, Ollama)는 `llm.Client` 인터페이스를 구현하므로 동일한 코드로 여러 모델을 교체하며 사용할 수 있습니다.

### 클라이언트 생성

```go
import (
    "github.com/wkqco33/LLM_client_go/openai"
    "github.com/wkqco33/LLM_client_go/ollama"
    "github.com/wkqco33/LLM_client_go/retry"
)

// OpenAI 생성 (재시도 정책 포함)
client := openai.New(openai.Config{
    APIKey:      os.Getenv("OPENAI_API_KEY"),
    RetryPolicy: &retry.DefaultPolicy,
})

// Ollama 생성 (로컬 실행, API 키 불필요)
// ollama.New는 OpenAI 호환 API를 그대로 감싼 *openai.Client를 반환합니다.
ollamaClient := ollama.New(ollama.Config{
    BaseURL: "http://localhost:11434/v1", // 생략 시 기본값
})
```

### 채팅 완성 (llm.Client 활용)

```go
import llm "github.com/wkqco33/LLM_client_go"

var myClient llm.Client = client // 또는 ollamaClient

resp, err := myClient.Complete(ctx, llm.ChatRequest{
    Model: "gpt-4o", // 또는 Ollama 사용 시 "llama3.2" 등
    Messages: []llm.Message{
        {Role: llm.RoleUser, Content: "Hello!"},
    },
})
```

---

## 3. 에이전트 및 자동 도구 실행

`agent.Runner`를 사용하면 모델의 도구 호출(Tool Calling) 요청을 자동으로 처리할 수 있습니다.

```go
import "github.com/wkqco33/LLM_client_go/agent"

// 1. 에이전트 생성
runner := agent.NewRunner(client, "gpt-4o", 
    agent.WithSystemPrompt("You are a helpful assistant."),
    agent.WithMaxHistory(5),
)

// 2. 실행 가능한 도구 등록 (agent.ExecutableTool 인터페이스 구현체)
runner.RegisterTool(&myWeatherTool{})

// 3. 실행 (최종 응답이 나올 때까지 모델-도구 간 상호작용 자동 수행)
msgs, finalResp, err := runner.Run(ctx, userMessages)
```

---

## 4. MCP (Model Context Protocol) 연동

외부 MCP 서버의 도구들을 에이전트에 즉시 연결할 수 있습니다.

### Stdio(JSON-RPC) 방식

```go
import "github.com/wkqco33/LLM_client_go/mcp"

// 1. 로컬 MCP 서버 프로세스 실행 (stdio 방식)
mcpProvider, _ := mcp.NewStdioClient("npx", "-y", "@modelcontextprotocol/server-filesystem", "/home/user/docs")
defer mcpProvider.Close()

// 2. MCP 도구 가져오기 및 에이전트 등록
mcpTools, _ := mcpProvider.ListTools(ctx)
for _, t := range mcp.WrapTools(mcpProvider, mcpTools) {
    runner.RegisterTool(t)
}

// 이제 에이전트는 파일 시스템 도구를 사용할 수 있습니다.
runner.Run(ctx, messages)
```

---

## 5. 고급 기능

### 자동 재시도 (Retry Policy)

네트워크 일시 오류나 429(Rate Limit) 발생 시 자동으로 백오프를 수행합니다.

```go
policy := retry.Policy{
    MaxRetries: 3,
    MinWait:    1 * time.Second,
}
client := openai.New(openai.Config{..., RetryPolicy: &policy})
```

### 토큰 계산기 (Token Counter)

전송 전 컨텍스트 크기를 미리 확인합니다.

```go
import "github.com/wkqco33/LLM_client_go/token"

count := token.Estimate("텍스트 토큰 수 예측")
msgCount := token.DefaultCounter.CountMessages(messages)
```

### Structured Outputs (JSON Schema 강제)

모델이 항상 특정 JSON 구조로만 응답하도록 강제합니다.

```go
req := llm.ChatRequest{
    ResponseFormat: &llm.ResponseFormat{
        Type: "json_schema",
        JSONSchema: &llm.JSONSchemaDef{
            Name:   "weather_result",
            Strict: true,
            Schema: map[string]any{...},
        },
    },
}
```

---

## 6. 에러 처리

`llm` 패키지는 상태 코드별 센티넬 에러를 제공하여 분기 처리를 용이하게 합니다.

```go
if err != nil {
    if errors.Is(err, llm.ErrRateLimited) {
        // 속도 제한 대응
    } else if errors.Is(err, llm.ErrUnauthorized) {
        // API 키 체크
    }
}
```

상세한 예제는 [examples/](../examples/) 디렉토리를 참고하세요.
