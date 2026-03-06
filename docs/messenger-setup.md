# 메신저 연결 가이드

Discord, Telegram, Slack 봇을 설정하고 LLM 클라이언트와 연결하는 단계별 가이드입니다.

## 목차

- [공통 준비 사항](#공통-준비-사항)
- [Discord 봇 설정](#discord-봇-설정)
- [Telegram 봇 설정](#telegram-봇-설정)
- [Slack 봇 설정](#slack-봇-설정)
- [환경변수 정리](#환경변수-정리)
- [여러 봇 동시 실행](#여러-봇-동시-실행)
- [LLM 백엔드 전환](#llm-백엔드-전환)
- [커스터마이징](#커스터마이징)

---

## 공통 준비 사항

### 1. LLM API 키 발급

**OpenAI 사용 시**
1. [platform.openai.com](https://platform.openai.com) 로그인
2. **API keys** → **Create new secret key**
3. 생성된 키를 `OPENAI_API_KEY` 환경변수에 설정

**Azure OpenAI 사용 시**
1. [Azure Portal](https://portal.azure.com) → **Azure OpenAI** 리소스 생성
2. **Keys and Endpoint** 탭에서 키와 엔드포인트 확인
3. **Model deployments** 탭에서 배포 이름 확인

### 2. 예제 실행 준비

```bash
# 소스 클론 후 의존성 설치
cd LLM_client_go
go mod download
```

---

## Discord 봇 설정

### 봇 애플리케이션 생성

1. [Discord Developer Portal](https://discord.com/developers/applications) 접속
2. **New Application** 클릭 → 이름 입력 후 생성
3. 좌측 메뉴 **Bot** 클릭 → **Add Bot** 클릭
4. **Token** 섹션에서 **Reset Token** → 토큰 복사

### 봇 권한 설정

**Bot** 탭 → **Privileged Gateway Intents** 에서 아래 항목 활성화:
- ✅ **Message Content Intent** (메시지 내용 읽기에 필수)

**OAuth2 → URL Generator** 탭:
- Scopes: `bot` 체크
- Bot Permissions: `Send Messages`, `Read Message History` 체크
- 생성된 URL로 접속하여 봇을 서버에 초대

### 봇 실행

```bash
export DISCORD_BOT_TOKEN="your-bot-token"
export OPENAI_API_KEY="your-openai-key"

go run examples/discord_bot/main.go
```

### Discord 봇 사용법

| 동작 | 방법 |
|---|---|
| 봇과 대화 | 봇이 있는 채널에서 메시지 전송 |
| DM 대화 | 봇에게 다이렉트 메시지 전송 |
| 대화 초기화 | `!reset` 입력 |

> **참고:** 봇은 자신이 있는 모든 채널의 메시지에 응답합니다.  
> 특정 채널에서만 응답하게 하려면 `bots/discord/bot.go`의 `onMessage`에서 채널 ID를 필터링하세요.

---

## Telegram 봇 설정

### BotFather로 봇 생성

1. Telegram에서 [@BotFather](https://t.me/BotFather) 검색 후 대화 시작
2. `/newbot` 명령어 입력
3. 봇 이름 입력 (예: `My LLM Bot`)
4. 봇 사용자명 입력 — `_bot`으로 끝나야 함 (예: `my_llm_helper_bot`)
5. 발급된 **HTTP API 토큰** 복사

### 봇 명령어 등록 (선택)

BotFather에서 `/setcommands` 입력 후 아래 내용 붙여넣기:

```
reset - 대화 기록을 초기화합니다
```

### 봇 실행

```bash
export TELEGRAM_BOT_TOKEN="123456789:ABCdefGHIjklMNOpqrSTUvwxyz"
export OPENAI_API_KEY="your-openai-key"

go run examples/telegram_bot/main.go
```

### Telegram 봇 사용법

| 동작 | 방법 |
|---|---|
| 봇과 대화 | 봇에게 메시지 전송 (DM 또는 그룹에서 초대 후 사용) |
| 대화 초기화 | `/reset` 입력 |

> **그룹 채팅 사용 시:** 그룹에서는 봇에게 직접 메시지를 보내거나 `@봇이름 메시지` 형식으로 멘션해서 사용하세요.  
> 그룹의 모든 메시지에 응답하게 하려면 BotFather에서 `/setprivacy` → `Disable`로 설정하세요.

---

## Slack 봇 설정

Slack 봇은 **Socket Mode**를 사용하므로 공개 서버 없이도 로컬에서 실행할 수 있습니다.

### Slack App 생성

1. [api.slack.com/apps](https://api.slack.com/apps) 접속 → **Create New App**
2. **From scratch** 선택
3. 앱 이름 입력, 설치할 워크스페이스 선택 → **Create App**

### Socket Mode 활성화

1. 좌측 메뉴 **Socket Mode** 클릭
2. **Enable Socket Mode** 토글 ON
3. App-Level Token 이름 입력 (예: `socket-token`)
4. **connections:write** 스코프 선택 → **Generate**
5. 생성된 `xapp-` 토큰 복사 → `SLACK_APP_TOKEN`

### Bot Token 발급

1. 좌측 메뉴 **OAuth & Permissions** 클릭
2. **Bot Token Scopes** 에서 아래 스코프 추가:
   - `app_mentions:read` — 멘션 읽기
   - `chat:write` — 메시지 전송
   - `im:history` — DM 메시지 읽기
   - `im:read` — DM 채널 접근
   - `im:write` — DM 전송
3. 상단 **Install to Workspace** → **Allow**
4. 생성된 `xoxb-` 토큰 복사 → `SLACK_BOT_TOKEN`

### Event Subscriptions 설정

1. 좌측 메뉴 **Event Subscriptions** 클릭
2. **Enable Events** 토글 ON
3. **Subscribe to bot events** 에서 아래 이벤트 추가:
   - `app_mention` — 채널에서 @멘션
   - `message.im` — DM 메시지

### 봇 실행

```bash
export SLACK_BOT_TOKEN="xoxb-your-bot-token"
export SLACK_APP_TOKEN="xapp-your-app-token"
export OPENAI_API_KEY="your-openai-key"

go run examples/slack_bot/main.go
```

### Slack 봇 사용법

| 동작 | 방법 |
|---|---|
| 채널에서 대화 | `@봇이름 메시지` 형식으로 멘션 |
| DM 대화 | 봇에게 다이렉트 메시지 전송 |
| 대화 초기화 | `!reset` 입력 |

> **워크스페이스에 봇 추가하기:** Slack 앱에서 채널 → **Integrations** → **Add apps** → 봇 이름 검색 후 추가

---

## 환경변수 정리

| 변수명 | 설명 | 필수 |
|---|---|---|
| `OPENAI_API_KEY` | OpenAI API 키 | OpenAI 사용 시 |
| `OPENAI_MODEL` | 사용할 모델 (기본값: `gpt-4o`) | 선택 |
| `AZURE_OPENAI_ENDPOINT` | Azure OpenAI 엔드포인트 URL | Azure 사용 시 |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI API 키 | Azure 사용 시 |
| `AZURE_OPENAI_DEPLOYMENT` | Azure 배포 이름 | Azure 사용 시 |
| `BACKEND` | LLM 백엔드 (`azure` 또는 기본값 openai) | 선택 |
| `DISCORD_BOT_TOKEN` | Discord 봇 토큰 | Discord 사용 시 |
| `TELEGRAM_BOT_TOKEN` | Telegram 봇 토큰 | Telegram 사용 시 |
| `SLACK_BOT_TOKEN` | Slack 봇 OAuth 토큰 (`xoxb-`) | Slack 사용 시 |
| `SLACK_APP_TOKEN` | Slack 앱 레벨 토큰 (`xapp-`) | Slack 사용 시 |

### `.env` 파일 예시

```bash
# LLM Backend
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4o

# Discord
DISCORD_BOT_TOKEN=MTI...

# Telegram
TELEGRAM_BOT_TOKEN=123456789:ABC...

# Slack
SLACK_BOT_TOKEN=xoxb-...
SLACK_APP_TOKEN=xapp-...
```

> **보안 주의:** `.env` 파일은 절대 Git에 커밋하지 마세요.  
> `.gitignore`에 `.env`를 추가하세요.

---

## 여러 봇 동시 실행

여러 플랫폼 봇을 하나의 프로그램에서 실행하려면 `errgroup`과 `context`를 조합합니다.

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"

    "llm-client-go/bots"
    discordbot "llm-client-go/bots/discord"
    telegrambot "llm-client-go/bots/telegram"
    "golang.org/x/sync/errgroup"
)

func main() {
    backend := bots.NewOpenAIBackend(os.Getenv("OPENAI_API_KEY"), "gpt-4o")
    sessions := bots.NewSessionManager(bots.WithSystemPrompt("You are helpful."))

    discord, _ := discordbot.New(discordbot.Config{
        Token:    os.Getenv("DISCORD_BOT_TOKEN"),
        Backend:  backend,
        Sessions: sessions, // 세션 공유도 가능
    })
    telegram, _ := telegrambot.New(telegrambot.Config{
        Token:    os.Getenv("TELEGRAM_BOT_TOKEN"),
        Backend:  backend,
        Sessions: sessions,
    })

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    g, ctx := errgroup.WithContext(ctx)
    g.Go(func() error { return discord.Start(ctx) })
    g.Go(func() error { return telegram.Start(ctx) })

    if err := g.Wait(); err != nil {
        log.Fatal(err)
    }
}
```

---

## LLM 백엔드 전환

모든 예제는 `BACKEND` 환경변수로 OpenAI ↔ Azure를 전환할 수 있습니다.

```bash
# OpenAI 사용 (기본값)
go run examples/discord_bot/main.go

# Azure OpenAI 사용
BACKEND=azure \
AZURE_OPENAI_ENDPOINT=https://my-resource.openai.azure.com \
AZURE_OPENAI_API_KEY=... \
AZURE_OPENAI_DEPLOYMENT=gpt-4o \
go run examples/discord_bot/main.go
```

코드에서 직접 지정할 수도 있습니다.

```go
// OpenAI
backend := bots.NewOpenAIBackend(apiKey, "gpt-4o")

// Azure OpenAI
backend := bots.NewAzureBackend(endpoint, apiKey, "my-deployment")

// 커스텀 백엔드 — Backend 인터페이스를 직접 구현
type MyBackend struct{}
func (b *MyBackend) Complete(ctx context.Context, messages []llm.Message) (string, error) {
    // 원하는 LLM 호출 로직
}
```

---

## 커스터마이징

### 시스템 프롬프트 변경

```go
sessions := bots.NewSessionManager(
    bots.WithSystemPrompt("You are a customer support agent for Acme Corp. Be concise and professional."),
    bots.WithMaxHistory(30),
)
```

### 대화 히스토리 길이 조정

```go
// 최근 10개 메시지만 유지 (메모리 절약)
bots.WithMaxHistory(10)

// 제한 없음 (장기 대화 필요 시, 메모리 주의)
bots.WithMaxHistory(0)
```

### Discord 특정 채널에서만 응답

`bots/discord/bot.go`의 `onMessage` 함수에 채널 필터를 추가합니다.

```go
func (b *Bot) onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
    // 특정 채널 ID에서만 응답
    allowedChannels := map[string]bool{
        "123456789012345678": true,
    }
    if !allowedChannels[m.ChannelID] {
        return
    }
    // ... 기존 로직
}
```

### 응답 온도(Temperature) 조정

`bots/handler.go`의 `OpenAIBackend.Complete`에서 Temperature를 설정합니다.

```go
func (b *OpenAIBackend) Complete(ctx context.Context, messages []llm.Message) (string, error) {
    temp := 0.7
    resp, err := b.Client.Chat.Complete(ctx, openai.ChatRequest{
        Model:       b.Model,
        Messages:    messages,
        Temperature: &temp,
    })
    // ...
}
```
