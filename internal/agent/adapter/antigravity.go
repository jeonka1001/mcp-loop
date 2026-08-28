package adapter

import (
	"time"

	"mcp-loop/internal/agent/entity"
	"mcp-loop/internal/shared/apperr"
	"mcp-loop/internal/shared/process"
)

const antigravityID = "antigravity"

// Antigravity는 agy(Antigravity) CLI 어댑터다. Antigravity IDE에 로그인된 구독 세션을 사용한다.
type Antigravity struct{}

func (Antigravity) ID() string    { return antigravityID }
func (Antigravity) Label() string { return "Antigravity CLI (agy, 구독)" }

func (Antigravity) Defaults() Defaults {
	return Defaults{Command: "agy", Timeout: 5 * time.Minute, Env: map[string]string{}}
}

func (Antigravity) BlockedEnvKeys() []string {
	return []string{
		"GEMINI_API_KEY",
		"GOOGLE_API_KEY",
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
	}
}

func (Antigravity) AuthNote() string {
	return "Antigravity IDE(또는 `agy install`)로 로그인된 구독 세션을 사용합니다. " +
		"`agy` 바이너리는 보통 ~/.local/bin에 설치되므로 PATH에 추가하세요. " +
		"개인 환경의 절대 경로가 필요하면 커밋되는 config/agents.json 대신 MCP_LOOP_CONFIG로 가리키는 개인 설정 파일에 지정하세요. " +
		"파일 수정 방지를 위해 툴 권한을 자동 승인하지 않으므로, cwd 파일을 직접 읽어야 하는 프롬프트는 빈 응답으로 실패할 수 있다 — 필요한 내용을 prompt에 직접 담을 것."
}

// BuildArgs는 agy CLI 어댑터의 인자를 만든다.
// agy -p/--print는 stdin이 아니라 인자 값으로 프롬프트를 받으므로(다른 어댑터와 달리 stdin을 지원하지 않음),
// 여기서는 예외적으로 query.Prompt를 인자에 직접 담는다.
// --dangerously-skip-permissions는 쓰지 않는다: agy는 파일을 읽을 때조차 run_command 툴로 넘어가는 경우가 있는데,
// 헤드리스 실행은 그 권한 확인 프롬프트에 응답할 수 없어 자동 거부되고 빈 응답만 돌아온다.
// 이 거부는 안전하게 실패할 뿐이므로(파일이 실제로 바뀌지는 않는다), 리뷰처럼 프롬프트에 필요한 내용을 다 담아 보내는
// 용도로 한정하고 codex(read-only 샌드박스)와 동일하게 "파일을 고칠 수 없는" 상태를 유지한다.
func (Antigravity) BuildArgs(cfg entity.Config, query entity.Query) []string {
	args := []string{"--print", query.Prompt, "--output-format", "json", "--mode", "plan"}
	args = withModel(args, cfg, query)
	return append(args, cfg.ExtraArgs...)
}

func (Antigravity) Parse(result process.RunResult) (Parsed, error) {
	body := decodeObject(result.Stdout)
	if body == nil {
		return Parsed{}, apperr.Executionf(antigravityID, failureDetail(result), "Antigravity CLI 출력에서 JSON 응답을 찾지 못했습니다.")
	}
	if err := antigravityError(body, result); err != nil {
		return Parsed{}, err
	}
	text := pickString(body, "response")
	if text == "" {
		return Parsed{}, apperr.Executionf(antigravityID, failureDetail(result), "Antigravity CLI 응답에 response 필드가 없습니다.")
	}
	return Parsed{Text: text, Usage: antigravityUsage(body)}, nil
}

func antigravityError(body map[string]any, result process.RunResult) error {
	if pickString(body, "status") != "ERROR" {
		return nil
	}
	message := pickString(body, "error")
	if message == "" {
		message = "알 수 없는 오류"
	}
	return apperr.Executionf(antigravityID, failureDetail(result), "Antigravity CLI 오류: %s", message)
}

func antigravityUsage(body map[string]any) *entity.Usage {
	usage := nestedMap(body, "usage")
	if usage == nil {
		return nil
	}
	return hasUsage(&entity.Usage{
		InputTokens:       pickInt(usage, "input_tokens"),
		OutputTokens:      pickInt(usage, "output_tokens"),
		CachedInputTokens: pickInt(usage, "cache_read_tokens"),
	})
}
