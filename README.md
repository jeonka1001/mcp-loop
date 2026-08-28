# mcp-loop

구독 이용권(OAuth 세션)으로 로그인된 **Gemini / Codex / Claude CLI**에 질의를 대신 던져주는 MCP 서버.
API 키 과금 경로를 쓰지 않으며, 호출할 에이전트는 MCP 클라이언트가 **명시적으로 선택**한다.

Go 단일 바이너리로 빌드된다. 실행에 런타임 의존성이 없다.

## 동작 원리

각 벤더 SDK를 호출하지 않고, 이미 로그인된 CLI를 비대화형 모드로 자식 프로세스 실행한다.

| 에이전트 | 실행 명령 | 사용하는 자격증명 |
|---|---|---|
| `gemini` | `gemini --output-format json` | `~/.gemini/oauth_creds.json` (Google OAuth) |
| `codex`  | `codex exec --json --sandbox read-only --skip-git-repo-check -` | `~/.codex/auth.json` (`auth_mode: chatgpt`) |
| `claude` | `claude --print --output-format json` | `~/.claude` (Pro/Max 로그인 세션) |

프롬프트는 argv가 아닌 **stdin**으로 전달한다(길이 제한·셸 이스케이프 회피).

### API 키 폴백 차단

CLI를 실행할 때 상속 환경변수에서 아래 키를 **제거**한다. 구독 세션이 만료돼도 API 키로 조용히 넘어가 과금되는 일이 없다.

- gemini: `GEMINI_API_KEY`, `GOOGLE_API_KEY`, `GOOGLE_GENAI_USE_VERTEXAI`, `GOOGLE_GENAI_USE_GCA`, `GOOGLE_APPLICATION_CREDENTIALS`
- codex: `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `CODEX_API_KEY`
- claude: `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_API_KEY_HELPER`, `CLAUDE_CODE_USE_BEDROCK`, `CLAUDE_CODE_USE_VERTEX`

## 사전 준비

Go 1.25 이상이 필요하다.

```bash
npm i -g @google/gemini-cli @openai/codex @anthropic-ai/claude-code
```

각 CLI를 한 번씩 실행해 로그인해 둔다 (`gemini`, `codex login`, `claude`).
로그인하지 않은 에이전트는 `list_agents`에서 상태로 드러나며, 호출 시 안내 메시지와 함께 실패한다.

## 빌드

```bash
make build
```

## MCP 클라이언트 등록

### 전역 등록 (권장)

Claude Code CLI라면 `claude mcp add`로 user 스코프에 등록한다. 별도 설정 파일 없이 이후 모든 프로젝트·세션에서 자동으로 연결된다.

```bash
claude mcp add -s user mcp-loop -- /path/to/mcp-loop/bin/mcp-loop
```

등록 확인:

```bash
claude mcp list
# mcp-loop: /path/to/mcp-loop/bin/mcp-loop  - ✔ Connected
```

이미 실행 중인 세션에는 반영되지 않는다. 새 세션을 열어야 툴이 노출된다.

### 프로젝트 단위 등록

특정 프로젝트에서만 쓰려면 해당 프로젝트 루트의 `.mcp.json`(Claude Code) 또는 클라이언트의 MCP 설정에 바이너리 절대 경로를 넣는다.

```json
{
  "mcpServers": {
    "mcp-loop": {
      "command": "/path/to/mcp-loop/bin/mcp-loop"
    }
  }
}
```

두 방식 모두 설정 파일은 바이너리 기준 `../config/agents.json`을 자동으로 찾는다. 다른 경로를 쓰려면 `MCP_LOOP_CONFIG` 환경변수를 지정한다.

## 제공 툴

### `list_agents`
등록된 에이전트의 설치 여부, 활성화 상태, 기본 모델, 인증 안내를 반환한다. 인자 없음.

### `ask_agent`
지정한 **한 개** 에이전트에만 질의한다.

| 인자 | 필수 | 설명 |
|---|---|---|
| `agent` | ✅ | `gemini` \| `codex` \| `claude` (스키마 enum으로 강제) |
| `prompt` | ✅ | 질문/지시문 (최대 200,000자) |
| `model` | | CLI가 지원하는 모델명. 미지정 시 CLI 기본값 |
| `timeout_ms` | | 5,000 ~ 900,000 |
| `cwd` | | CLI 실행 작업 디렉터리 |

### `ask_agents`
`agents` 배열에 담긴 에이전트에만 같은 질의를 병렬 전달한다. 개별 성공/실패가 따로 표시되며, 일부가 실패해도 나머지 결과는 반환된다.

> `agent` / `agents`는 필수이며 기본값이 없다. 입력 스키마에서 `required` + `minItems: 1`로 강제되므로 **모든 에이전트를 자동으로 호출하는 경로는 존재하지 않는다.**

## 설정 — `config/agents.json`

```json
{
  "defaults": { "timeoutMs": 180000, "maxConcurrency": 3 },
  "agents": {
    "gemini": { "enabled": true, "command": "gemini", "model": null, "timeoutMs": 180000, "env": {} }
  }
}
```

| 키 | 설명 |
|---|---|
| `enabled` | false면 호출이 거부된다 |
| `command` | CLI 실행 파일명 또는 절대 경로 |
| `extraArgs` | 어댑터가 만든 인자 뒤에 덧붙일 추가 인자 |
| `model` | 해당 에이전트의 기본 모델 |
| `timeoutMs` | 에이전트별 기본 타임아웃 |
| `env` | CLI에 주입할 환경변수 |
| `defaults.maxConcurrency` | `ask_agents` 동시 실행 수 |

## 테스트

```bash
make test
```

인메모리 전송으로 서버-클라이언트를 붙여 툴 노출과 입력 거부 규칙을 검증한다. 실제 CLI는 호출하지 않는다.

실제 에이전트까지 확인하려면 스모크 도구를 쓴다.

```bash
make smoke                                   # 핸드셰이크 + 툴 목록 + list_agents
make smoke AGENTS=codex                      # codex에 실제 질의
make smoke AGENTS=codex,gemini PROMPT="질문"  # 다중 에이전트 질의
```

## 구조 (DDD)

```
cmd/mcp-loop/main.go                       MCP stdio 부트스트랩
cmd/smoke/main.go                          실제 CLI까지 확인하는 스모크 도구
internal/agent/
  entity/agent.go                          데이터 모델 + 입력 검증 규칙
  service/registry.go                      어댑터 기본값 + 설정 병합, 설치/활성 판정
  service/query.go                         단일/다중 질의 오케스트레이션, 동시성 제어
  controller/controller.go                 MCP 툴 진입점 (service에 위임만)
  controller/schema.go                     툴 입력 스키마 (agent enum 생성)
  adapter/adapter.go                        Adapter 인터페이스 + JSON 파싱 헬퍼
  adapter/{gemini,codex,claude}.go         CLI 인자 생성 + 출력 파싱
internal/shared/
  process/runner.go                        실행, env 새니타이즈, 타임아웃(SIGTERM→SIGKILL)
  config/config.go                         설정 로더
  apperr/errors.go                         도메인 에러
```

에이전트를 추가하려면 `adapter` 패키지에 `Adapter` 구현을 만들고 `cmd/mcp-loop/main.go`의 어댑터 슬라이스에 등록하면 된다. 툴 개수는 늘지 않고, `agent` enum에 자동으로 반영된다.

## 트러블슈팅

**`ProjectIdRequiredError` (gemini, exit 41)**
계정이 Google Cloud 프로젝트를 요구하는 경우다. 프로젝트 ID를 설정에 넣는다.

```json
{ "agents": { "gemini": { "env": { "GOOGLE_CLOUD_PROJECT": "your-project-id" } } } }
```

**CLI를 PATH에서 찾을 수 없음**
MCP 클라이언트가 로그인 셸 PATH를 상속하지 않는 경우가 있다. `config/agents.json`의 `command`에 절대 경로를 지정한다.

## 한계

- 구독 rate limit은 CLI를 직접 쓸 때와 동일하게 공유된다. MCP를 경유해도 한도가 늘지 않는다.
- 대화 맥락을 유지하지 않는다. 매 호출이 독립 세션이다.
- codex는 `read-only` 샌드박스로 실행되어 파일을 수정하지 않는다.
