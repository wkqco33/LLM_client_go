# AGENTS.md

이 문서는 이 저장소에서 작업하는 AI 코딩 에이전트(Claude Code, Cursor, Aider, Codex, Antigravity 등)와
사람 기여자 모두가 따라야 할 개발 방식과 아키텍처 가이드를 정의합니다.

---

## 1. 핵심 개발 철학: TDD (Test-Driven Development)

이 프로젝트는 **엄격한 TDD(Test-Driven Development)** 원칙에 따라 개발됩니다. 새 기능 추가, 버그 수정, 성능 리팩터링 시 반드시 아래 사이클을 준수하세요:

```
[1. Red] -> 실패하는 테스트 작성 (`task test`로 어서션 실패 확인)
   ↓
[2. Green] -> 테스트를 통과시키는 최소한의 구현 작성 (`task test` 통과 확인)
   ↓
[3. Refactor] -> 코드 정리 및 성능 최적화 (테스트 통과 유지 확인)
```

1. **Red** — 원하는 동작이나 재현할 버그를 검증하는 테스트를 먼저 작성합니다.
   - 컴파일 에러가 아닌 실제 테스트 단언(Assertion) 실패로 레드가 떠야 합니다.
   - 신규 함수/타입의 시그니처만 먼저 정의해두고 로직을 비워두거나 의도적으로 틀리게 작성하여 실패를 확인합니다.
2. **Green** — 테스트를 통과시키는 최소한의 깔끔한 코드를 작성합니다.
3. **Refactor** — 테스트가 계속 통과하는 상태를 유지하며 구조 개선, 중복 제거, 성능 최적화를 진행합니다.

> **예외:** `Taskfile.yml`, `go.mod` 의존성 정리, 순수 문서 변경은 TDD 대상에서 제외됩니다.

---

## 2. 패키지 아키텍처 및 Seam(접점) 설계

```
LLM_client_go/
├── types.go                  # 공통 타입 및 llm.Client 인터페이스
├── helpers.go                # 메시지/도구 생성 헬퍼 및 도구 호출 조합
├── errors.go                 # 상태 코드별 센티넬 에러 (ErrUnauthorized, ErrRateLimited 등)
├── agent/                    # multi-turn 에이전트 자동화 루프 (Runner, ExecutableTool)
├── mcp/                      # Model Context Protocol 클라이언트 (HTTP, Stdio) 및 브릿지
├── retry/                    # 지수 백오프/Jitter 기반 자동 재시도 미들웨어
├── token/                    # 휴리스틱 토큰 계산기
├── openai/                   # OpenAI API 클라이언트 구현체
├── azure/                    # Azure OpenAI API 클라이언트 구현체
├── ollama/                   # Ollama 로컬 서버 연동 클라이언트 (OpenAI 호환 래퍼)
├── bots/                     # 멀티 플랫폼 봇 공통 핸들러 (SessionManager, HandleTurn)
│   ├── discord/              # Discord 봇 어댑터
│   ├── slack/                # Slack 봇 어댑터 (Socket Mode)
│   └── telegram/             # Telegram 봇 어댑터
├── internal/
│   ├── apierr/               # HTTP 상태 코드 ↔ 센티넬 에러 매핑
│   ├── sse/                  # SSE(Server-Sent Events) 스트리밍 파서
│   └── transport/            # 공통 HTTP 클라이언트 빌더 및 JSON 디코더
└── examples/                 # 프로바이더별 예제 및 봇 실행 진입점
```

### 주요 인터페이스 및 확장 접점 (Seams)
- **`llm.Client`**: 모든 LLM 프로바이더(OpenAI, Azure, Ollama 등)가 구현해야 하는 단일 인터페이스.
- **`agent.ExecutableTool`**: 에이전트가 호출할 수 있는 로컬 또는 원격 도구 추상화.
- **`mcp.Provider`**: 외부 MCP 서버와의 도구 목록 조회(`ListTools`) 및 실행(`CallTool`) 추상화.
- **`bots.Backend`**: 메신저 봇이 LLM 완성을 요청하는 백엔드 인터페이스. `bots.CommonBackend`가 `llm.Client`를 감싸 기본 제공.
- **`bots.HandleTurn`**: 세션 이력 관리, 리셋 명령어 처리, 백엔드 호출을 단일 함수로 중앙화하여 플랫폼별 중복 방지.

---

## 3. 테스트 관례 및 작성 가이드

- **표준 라이브러리만 사용**: `testify`, `assert` 등 외부 라이브러리를 사용하지 않고 `if got != want { t.Errorf(...) }` 스타일을 유지합니다.
- **핸드메이드 Mocking**: 외부 모킹 프레임워크 없이 인터페이스(`llm.Client`, `bots.Backend`, `agent.ExecutableTool`, `mcp.Provider`)를 만족하는 최소한의 구조체를 손으로 직접 작성합니다.
- **HTTP 테스트 헬퍼**: `httptest.NewServer` + `testServer(t, handler)` 패턴을 사용하며, 반드시 `t.Helper()`와 `t.Cleanup(srv.Close)`를 등록합니다.
- **재시도 무력화 (`noRetry`)**: 클라이언트는 기본적으로 재시도가 활성화되어 있으므로, 상태 코드(401, 429, 500 등) 검증 테스트에는 반드시 `RetryPolicy: noRetry` (`&retry.Policy{}`)를 설정하여 불필요한 백오프 대기를 방지합니다.
- **컨텍스트 취소 테스트 시 서버 블로킹 방지**: `httptest.Server` 핸들러에서 5초 대기(`time.After`)를 두면 `srv.Close()` 호출 시 서버 종료가 블로킹되어 전체 테스트가 느려집니다. 동기화 채널(`reqStarted`, `handlerDone`)을 사용하여 `Complete()` 반환 즉시 핸들러가 종료되도록 작성하세요.
- **테스트 명명 규칙**: `Test<대상>_<시나리오>_<기대결과>` (예: `TestComplete_Error_401_Unauthorized`, `TestSplitMessage_MultibyteUTF8`).
- **테이블 기반 테스트**: 입력/출력 케이스가 다수인 경우 `struct` 슬라이스와 `t.Run(tc.name, ...)`을 사용합니다.
- **벤치마크 테스트**: 성능 병목 지점이나 메모리 할당이 중요한 유틸리티(`token`, `sse`, `helpers.CollectToolCalls`, `bots.SplitMessage`)에는 `BenchmarkXxx(b *testing.B)`를 작성하여 `b.N` 루프 및 `b.ResetTimer()`로 측정합니다.

---

## 4. 테스트 및 빌드 명령어 (Task)

- `task test` — 전체 유닛 테스트 실행 (커밋/PR 전 필수)
- `task test:verbose` — 상세 테스트 출력
- `task test:coverage` — 커버리지 측정 및 `coverage.html` 생성
- `task --watch test` — 파일 변경 감지 시 자동 재실행 (TDD 루프 권장)
- `go test -bench=. -benchmem ./...` — 벤치마크 및 메모리 할당 측정
- `task lint` — `go vet ./...` 정적 분석
- `task fmt` — `gofmt -s -w .` 포맷팅
- `task build` — 봇 및 예제 바이너리 빌드 검증

---

## 5. 보안 및 저장소 위생 (Security & Hygiene)

- **민감 정보 커밋 금지**: 실제 API 키, 토큰, 비밀번호, 개인 식별 정보를 코드나 커밋 메시지에 포함하지 마세요.
- **더미 문자열 사용**: 테스트 및 예제에는 `"test-key"`, `"dummy-token"`, `"your_openai_api_key"` 등의 명백한 플레이스홀더만 사용합니다.
- **환경 변수 분리**: 로컬 실행 시 `.env` 파일과 `os.Getenv`을 활용하며, `.env`는 `.gitignore`에 등록되어 있어야 합니다.
- **대용량 파일 금지**: 바이너리 빌드 결과물(`bin/`), 커버리지 리포트(`*.html`, `*.out`), IDE 캐시(`.idea/`, `.vscode/`)가 커밋되지 않도록 `.gitignore` 상태를 확인하세요.

