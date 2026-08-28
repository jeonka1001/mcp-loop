package adapter

import (
	"time"

	"mcp-loop/internal/agent/entity"
	"mcp-loop/internal/shared/apperr"
	"mcp-loop/internal/shared/process"
)

const claudeID = "claude"

// Claude는 claude CLI 어댑터다. ~/.claude 의 Pro/Max 구독 로그인 세션을 사용한다.
type Claude struct{}

func (Claude) ID() string    { return claudeID }
func (Claude) Label() string { return "Claude Code CLI (Pro/Max 구독)" }

func (Claude) Defaults() Defaults {
	return Defaults{Command: "claude", Timeout: 5 * time.Minute, Env: map[string]string{}}
}

func (Claude) BlockedEnvKeys() []string {
	return []string{
		"ANTHROPIC_API_KEY",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_API_KEY_HELPER",
		"CLAUDE_CODE_USE_BEDROCK",
		"CLAUDE_CODE_USE_VERTEX",
	}
}

func (Claude) AuthNote() string {
	return "claude CLI의 구독 로그인 세션을 사용합니다. 미설치 시 `npm i -g @anthropic-ai/claude-code` 후 `claude` 실행으로 로그인하세요."
}

func (Claude) BuildArgs(cfg entity.Config, query entity.Query) []string {
	args := withModel([]string{"--print", "--output-format", "json"}, cfg, query)
	return append(args, cfg.ExtraArgs...)
}

func (Claude) Parse(result process.RunResult) (Parsed, error) {
	body := decodeObject(result.Stdout)
	if body == nil {
		return Parsed{}, apperr.Executionf(claudeID, failureDetail(result), "Claude CLI 출력에서 JSON 응답을 찾지 못했습니다.")
	}
	if err := claudeError(body, result); err != nil {
		return Parsed{}, err
	}
	text := pickString(body, "result")
	if text == "" {
		return Parsed{}, apperr.Executionf(claudeID, failureDetail(result), "Claude CLI 응답에 result 필드가 없습니다.")
	}
	return Parsed{Text: text, Usage: claudeUsage(body)}, nil
}

func claudeError(body map[string]any, result process.RunResult) error {
	isError, _ := body["is_error"].(bool)
	if !isError && pickString(body, "subtype") != "error" {
		return nil
	}
	message := pickString(body, "result")
	if message == "" {
		message = "알 수 없는 오류"
	}
	return apperr.Executionf(claudeID, failureDetail(result), "Claude CLI 오류: %s", message)
}

func claudeUsage(body map[string]any) *entity.Usage {
	usage := nestedMap(body, "usage")
	if usage == nil {
		return nil
	}
	return hasUsage(&entity.Usage{
		InputTokens:       pickInt(usage, "input_tokens"),
		OutputTokens:      pickInt(usage, "output_tokens"),
		CachedInputTokens: pickInt(usage, "cache_read_input_tokens"),
	})
}
