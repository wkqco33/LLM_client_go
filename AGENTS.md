# AGENTS.md

이 문서는 이 저장소에서 작업하는 AI 코딩 에이전트(Claude Code, Cursor, Aider, Codex 등)와
사람 기여자 모두가 따라야 할 개발 방식을 정의합니다.

## 개발 방식: TDD (Test-Driven Development)

이 프로젝트는 **TDD로 개발**합니다. 새 기능을 추가하거나 버그를 고칠 때:

1. **Red** — 원하는 동작을 검증하는 실패하는 테스트를 먼저 작성한다. `task test`로 실제로
   실패하는 것을 확인한다 (컴파일 에러가 아니라 어서션 실패로 레드가 떠야 함 — 새 타입/함수
   시그니처는 먼저 정의해두고, 로직만 비워두거나 틀리게 둔다).
2. **Green** — 테스트를 통과시키는 최소한의 구현을 작성한다.
3. **Refactor** — 테스트가 계속 통과하는 상태를 유지하며 정리한다. 각 단계마다 `task test`로
   확인한다.

버그 수정도 동일합니다: 버그를 재현하는 실패 테스트를 먼저 추가하고, 그다음 고칩니다.

### 예외

- 순수 설정/보일러플레이트 변경(`go.mod` 정리, 문서, `Taskfile.yml` 등)은 TDD 대상이 아닙니다.
- 탐색적 스파이크(설계를 확정하기 전 프로토타이핑)는 테스트 없이 작성해도 되지만, 커밋하기
  전에 테스트를 채워 넣습니다.

## 테스트 관례

기존 테스트 스위트(`openai/`, `azure/`, `ollama/`, `agent/`, `bots/`, 루트 `llm_test.go`)가
이미 확립한 패턴을 그대로 따릅니다:

- **표준 라이브러리만 사용** — `testify`, `go-test/deep` 같은 외부 단언(assertion) 라이브러리를
  쓰지 않습니다. `if got != want { t.Errorf(...) }` 스타일.
- **HTTP 프로바이더 테스트**: `httptest.NewServer` + 패키지별 `testServer(t, handler)
  (*Client, *httptest.Server)` 헬퍼 (`t.Helper()` + `t.Cleanup(srv.Close)`). 클라이언트는
  기본적으로 재시도가 적용되므로, 상태 코드나 타임아웃을 검증하는 테스트는 반드시
  `RetryPolicy: noRetry`(각 패키지에 이미 정의된 `&retry.Policy{}`)를 넘겨 백오프 대기 없이
  1회 시도로 고정합니다.
- **목(mock)은 손으로 작성한 구조체** — 목킹 프레임워크 없이, 인터페이스(`llm.Client`,
  `bots.Backend`, `agent.ExecutableTool`, `mcp.Provider`)를 만족하는 최소 구조체를 그때그때
  정의합니다 (`agent/agent_test.go`의 `mockClient`, `bots/handler_test.go`의 `mockBackend` 참고).
- **테스트 이름**: `Test<대상>_<시나리오>_<기대결과>` (예: `TestComplete_Error_401_Unauthorized`,
  `TestNew_WithTimeout_Option`).
- **여러 케이스는 테이블 기반 + `t.Run` 서브테스트** (`llm_test.go`의
  `TestSentinelErrors_ErrorsIs` 참고).

## 테스트 실행

- `task test` — 전체 테스트 (커밋 전 필수)
- `task test:verbose` — 상세 출력
- `task test:coverage` — 커버리지 리포트 (`coverage.html`)
- `task --watch test` — 변경 감지 시 자동 재실행 (TDD 루프용)
- `task test:race`는 CGO가 필요합니다. CGO를 쓸 수 없는 환경에서는 실패하니, 사용 전에
  `CGO_ENABLED`을 확인하세요.

## 테스트 커버리지 현황

주요 패키지의 단위 테스트가 모두 구현되어 있습니다:

- `retry/` — 무재시도, 500 에러 재시도 한도, 서비스 복구, 비재시도 상태, Request Body 보존, 컨텍스트 취소
- `token/` — 휴리스틱 토큰 계산 및 도구 호출/메시지 오버헤드 산정
- `internal/` (`apierr`, `sse`, `transport`) — 에러 래핑, SSE 스트림 파싱/취소, HTTP 클라이언트 빌더 및 JSON 디코딩
- `mcp/` — HTTP 클라이언트, 도구 래핑 브릿지, Stdio 클라이언트 JSON-RPC 서브프로세스 통신
- `bots/` (`discord`, `slack`, `telegram`) — 클라이언트 초기화 유효성 검사 및 목킹 서버 연동


## 테스트하기 어려운 지점 (알려진 제약 — 손대기 전에 먼저 판단할 것)

- `mcp.StdioClient`는 `os/exec`를 직접 사용합니다. 순수 유닛 테스트를 하려면 실제
  서브프로세스(테스트용 fixture 스크립트)가 필요하거나, exec 실행을 인터페이스 뒤로 옮기는
  작은 리팩터링이 선행되어야 할 수 있습니다.
- `bots/discord`, `bots/slack`, `bots/telegram`은 각 플랫폼 SDK의 구체 타입
  (`discordgo.Session` 등)에 직접 의존합니다. `onMessage`/`handleMessage`의 로직(리셋 명령
  처리, 청킹)을 SDK 없이 단위 테스트하려면 "메시지 전송"만 분리한 작은 인터페이스가 먼저
  필요할 수 있습니다.

이런 곳은 "테스트를 어떻게 뺄지"부터가 설계 결정이므로, 씨딩(seam)을 어떻게 낼지 먼저
합의하고 나서 Red 단계로 들어가세요.
