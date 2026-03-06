# LLM Client Go

Go로 구현된 OpenAI / Azure OpenAI 클라이언트 라이브러리입니다.  
Chat Completions, SSE 스트리밍, Function Calling을 지원하며, Discord · Telegram · Slack 봇 어댑터를 내장하고 있습니다.

## 목차

- [특징](#특징)
- [패키지 구조](#패키지-구조)
- [빠른 시작](#빠른-시작)
  - [OpenAI Chat Completions](#openai-chat-completions)
  - [SSE 스트리밍](#sse-스트리밍)
  - [Function Calling](#function-calling)
  - [Azure OpenAI](#azure-openai)
- [봇 어댑터](#봇-어댑터)
- [세션 관리](#세션-관리)
- [에러 처리](#에러-처리)
- [메신저 연결 가이드](#메신저-연결-가이드)

---

## 특징

| 기능 | 설명 |
|---|---|
| **멀티 프로바이더** | OpenAI, Azure OpenAI |
| **Chat Completions** | 비스트리밍 / SSE 스트리밍 |
| **Function Calling** | Tool 정의, 응답 파싱, 스트림 조합 헬퍼 |
| **봇 어댑터** | Discord, Telegram, Slack (Socket Mode) |
| **세션 관리** | 유저별 대화 히스토리, MaxHistory 자동 트리밍 |
| **에러 처리** | HTTP 상태 코드별 구조화된 에러 타입 |
| **외부 의존성** | LLM 코어는 표준 라이브러리만 사용 |

---

## 패키지 구조

```
llm-client-go/
├── types.go                  # 공통 타입 (Message, Role, Tool, ToolCall …)
├── errors.go                 # APIError, 센티넬 에러
│
├── openai/
│   ├── client.go             # OpenAI 클라이언트 (Functional Options)
│   ├── chat.go               # Chat Completions
│   ├── stream.go             # SSE 스트리밍
│   └── tools.go              # Function Calling 헬퍼
│
├── azure/
│   ├── client.go             # Azure OpenAI 클라이언트 (api-key 인증)
│   ├── chat.go               # Chat Completions (DeploymentName 기반)
│   ├── stream.go             # SSE 스트리밍
│   └── tools.go              # Function Calling 헬퍼
│
├── bots/
│   ├── bot.go                # Bot 인터페이스
│   ├── session.go            # 유저별 세션 관리 (thread-safe)
│   ├── handler.go            # Backend 인터페이스 + OpenAI/Azure 구현체
│   ├── discord/bot.go        # Discord 봇 어댑터
│   ├── telegram/bot.go       # Telegram 봇 어댑터
│   └── slack/bot.go          # Slack 봇 어댑터 (Socket Mode)
│
└── examples/
    ├── openai_chat/          # 기본 채팅
    ├── openai_stream/        # 스트리밍
    ├── openai_tools/         # Function Calling
    ├── azure_chat/           # Azure 채팅
    ├── azure_stream/         # Azure 스트리밍
    ├── discord_bot/          # Discord 봇 실행
    ├── telegram_bot/         # Telegram 봇 실행
    └── slack_bot/            # Slack 봇 실행
```

---

## 빠른 시작

### OpenAI Chat Completions

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    llm "llm-client-go"
    "llm-client-go/openai"
)

func main() {
    client := openai.New(openai.Config{
        APIKey: os.Getenv("OPENAI_API_KEY"),
    })

    resp, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
        Model: "gpt-4o",
        Messages: []llm.Message{
            openai.NewSystemMessage("You are a helpful assistant."),
            openai.NewUserMessage("What is the capital of France?"),
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
    fmt.Printf("Tokens used: %d\n", resp.Usage.TotalTokens)
}
```

### SSE 스트리밍

```go
stream, err := client.Chat.Stream(ctx, openai.ChatRequest{
    Model:    "gpt-4o",
    Messages: []llm.Message{openai.NewUserMessage("Tell me a story.")},
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

for {
    chunk, err := stream.Next()
    if err != nil {
        log.Fatal(err)
    }
    if chunk == nil { // [DONE]
        break
    }
    for _, choice := range chunk.Choices {
        fmt.Print(choice.Delta.Content)
    }
}
```

### Function Calling

```go
weatherTool := openai.NewTool("get_weather", "Get the current weather", map[string]any{
    "type": "object",
    "properties": map[string]any{
        "city": map[string]any{"type": "string"},
    },
    "required": []string{"city"},
})

resp, err := client.Chat.Complete(ctx, openai.ChatRequest{
    Model:    "gpt-4o",
    Messages: []llm.Message{openai.NewUserMessage("Weather in Tokyo?")},
    Tools:    []llm.Tool{weatherTool},
})

if resp.Choices[0].FinishReason == "tool_calls" {
    for _, tc := range resp.Choices[0].Message.ToolCalls {
        // tc.Function.Name, tc.Function.Arguments 를 활용해 도구 실행
    }
}
```

### Azure OpenAI

```go
client := azure.New(azure.Config{
    Endpoint: "https://my-resource.openai.azure.com",
    APIKey:   os.Getenv("AZURE_OPENAI_API_KEY"),
    // APIVersion 기본값: "2024-02-01"
})

resp, err := client.Chat.Complete(ctx, azure.ChatRequest{
    DeploymentName: "my-gpt4o-deployment",
    Messages: []llm.Message{
        azure.NewUserMessage("Hello!"),
    },
})
```

---

## 봇 어댑터

모든 봇 어댑터는 동일한 패턴을 따릅니다.

```go
// 1. LLM 백엔드 선택
backend := bots.NewOpenAIBackend(os.Getenv("OPENAI_API_KEY"), "gpt-4o")
// 또는 Azure:
// backend := bots.NewAzureBackend(endpoint, apiKey, deploymentName)

// 2. 세션 관리자 설정
sessions := bots.NewSessionManager(
    bots.WithSystemPrompt("You are a helpful assistant."),
    bots.WithMaxHistory(20),
)

// 3. 봇 생성 + 실행
bot, _ := discord.New(discord.Config{Token: "...", Backend: backend, Sessions: sessions})

ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()
bot.Start(ctx)
```

각 플랫폼별 설정은 [메신저 연결 가이드](docs/messenger-setup.md)를 참고하세요.

### 리셋 명령어

| 플랫폼 | 명령어 |
|---|---|
| Discord | `!reset` |
| Telegram | `/reset` |
| Slack | `!reset` |

---

## 세션 관리

```go
sm := bots.NewSessionManager(
    bots.WithSystemPrompt("You are a pirate."), // 고정 시스템 프롬프트
    bots.WithMaxHistory(20),                    // 유저당 최대 메시지 수
)

sm.Append("user-id", llm.Message{Role: llm.RoleUser, Content: "Ahoy!"})
history := sm.GetHistory("user-id") // 시스템 메시지 포함 전체 히스토리
sm.Reset("user-id")                 // 대화 초기화
```

- **MaxHistory** 초과 시 오래된 메시지부터 자동 제거
- 시스템 메시지는 트리밍 대상에서 제외되어 항상 유지됨
- `sync.RWMutex` 기반으로 동시 접근에 안전

---

## 에러 처리

```go
_, err := client.Chat.Complete(ctx, req)
if err != nil {
    var apiErr *llm.APIError
    if llm.IsAPIError(err, &apiErr) {
        fmt.Printf("API error %d: %s\n", apiErr.StatusCode, apiErr.Message)
    }

    // 센티넬 에러로 분기
    switch {
    case errors.Is(err, llm.ErrUnauthorized):
        // API 키 확인
    case errors.Is(err, llm.ErrRateLimited):
        // 재시도 로직
    case errors.Is(err, llm.ErrServerError):
        // 서버 오류 처리
    }
}
```

### 에러 종류

| 에러 | HTTP 상태 | 설명 |
|---|---|---|
| `ErrUnauthorized` | 401 | API 키 오류 |
| `ErrRateLimited` | 429 | 요청 한도 초과 |
| `ErrNotFound` | 404 | 리소스 없음 |
| `ErrBadRequest` | 400 | 잘못된 요청 |
| `ErrServerError` | 5xx | 서버 오류 |
| `ErrStreamClosed` | — | 닫힌 스트림 읽기 시도 |

---

## 메신저 연결 가이드

자세한 플랫폼별 설정 방법은 [`docs/messenger-setup.md`](docs/messenger-setup.md)를 참고하세요.
