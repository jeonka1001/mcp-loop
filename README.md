# mcp-loop

Codex / Claude / Antigravity CLI의 **구독 세션**을 그대로 재사용해 질의를 중계하는 MCP 서버. **API 키 종량 과금 없이, 이미 로그인된 CLI를 비대화형 서브프로세스로 실행한다.**

## 왜 만들었나

여러 CLI 구독(ChatGPT Plus, Claude Pro, Antigravity)을 갖고 있는데도 MCP로 묶으려면 종량제 API 키를 또 써야 하는 게 낭비라고 생각했다. 대신 이미 인증된 CLI 세션을 그대로 재사용하는 방식을 택했다.

## 설계에서 신경 쓴 부분

- **API 키 폴백 차단**: 서브프로세스 환경변수에서 API 키 관련 변수를 명시적으로 제거해, 구독이 만료돼도 조용히 종량 과금으로 넘어가는 걸 원천 차단한다.
- **어댑터 패턴**: JSON stdout, JSONL 이벤트 스트림, 인자 기반 프롬프트 등 프로토콜이 전혀 다른 CLI 3종을 단일 `Adapter` 인터페이스로 통합했다. 에이전트 추가는 파일 1개 + 등록 1줄.
- **안전한 프로세스 관리**: SIGTERM→SIGKILL 타임아웃, 출력 버퍼 상한, "자동 전체 호출" 경로 자체를 없애 명시적으로 선택한 에이전트만 호출되도록 강제했다.
- **실전 트러블슈팅**: Google이 Gemini CLI 개인 티어를 서비스 정책으로 막았을 때 대체 에이전트(Antigravity)를 발굴해 통합했고, 그 CLI의 헤드리스 권한 이슈도 내부 로그를 직접 분석해 원인을 규명하고 절충안을 설계했다.

## 구조

`entity`(도메인 모델) → `adapter`(CLI별 프로토콜 변환) → `service`(오케스트레이션) → `controller`(MCP 진입점)로 계층을 분리했다.

## Stack

Go · [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)

## 실행

```bash
make build
claude mcp add -s user mcp-loop -- $(pwd)/bin/mcp-loop
```

`list_agents` / `ask_agent` / `ask_agents` 3개 MCP 툴을 노출한다.
