package adapter

import (
	"strings"
	"time"

	"mcp-loop/internal/agent/entity"
	"mcp-loop/internal/shared/apperr"
	"mcp-loop/internal/shared/process"
)

const codexID = "codex"

// Codex는 codex CLI 어댑터다. ~/.codex/auth.json 의 ChatGPT 구독 세션(auth_mode=chatgpt)을 사용한다.
type Codex struct{}

func (Codex) ID() string    { return codexID }
func (Codex) Label() string { return "Codex CLI (ChatGPT 구독)" }

func (Codex) Defaults() Defaults {
	return Defaults{Command: "codex", Timeout: 5 * time.Minute, Env: map[string]string{}}
}

func (Codex) BlockedEnvKeys() []string {
	return []string{"OPENAI_API_KEY", "OPENAI_BASE_URL", "CODEX_API_KEY"}
}

func (Codex) AuthNote() string {
	return "codex CLI의 ChatGPT 로그인 세션을 사용합니다. `codex login` 상태와 ~/.codex/auth.json의 auth_mode=chatgpt를 확인하세요."
}

// BuildArgs는 read-only 샌드박스로 고정하고, git 저장소가 아닌 곳에서도 실행되도록 한다.
// 마지막 "-"는 프롬프트를 stdin에서 읽으라는 의미다.
func (Codex) BuildArgs(cfg entity.Config, query entity.Query) []string {
	args := []string{"exec", "--json", "--sandbox", "read-only", "--skip-git-repo-check"}
	args = withModel(args, cfg, query)
	return append(append(args, cfg.ExtraArgs...), "-")
}

func (Codex) Parse(result process.RunResult) (Parsed, error) {
	events := decodeLines(result.Stdout)
	if len(events) == 0 {
		return Parsed{}, apperr.Executionf(codexID, failureDetail(result), "Codex CLI 출력에서 JSONL 이벤트를 찾지 못했습니다.")
	}
	if err := codexError(events, result); err != nil {
		return Parsed{}, err
	}
	text := codexText(events)
	if text == "" {
		return Parsed{}, apperr.Executionf(codexID, failureDetail(result), "Codex CLI 응답에 agent_message가 없습니다.")
	}
	return Parsed{Text: text, Usage: codexUsage(events)}, nil
}

func codexError(events []map[string]any, result process.RunResult) error {
	for _, event := range events {
		if pickString(event, "type") != "error" {
			continue
		}
		message := pickString(event, "message")
		if message == "" {
			message = "알 수 없는 오류"
		}
		return apperr.Executionf(codexID, failureDetail(result), "Codex CLI 오류: %s", message)
	}
	return nil
}

func codexText(events []map[string]any) string {
	var messages []string
	for _, event := range events {
		if pickString(event, "type") != "item.completed" {
			continue
		}
		item := nestedMap(event, "item")
		if item != nil && pickString(item, "type") == "agent_message" {
			messages = append(messages, pickString(item, "text"))
		}
	}
	return strings.TrimSpace(strings.Join(messages, "\n\n"))
}

func codexUsage(events []map[string]any) *entity.Usage {
	for i := len(events) - 1; i >= 0; i-- {
		if pickString(events[i], "type") != "turn.completed" {
			continue
		}
		usage := nestedMap(events[i], "usage")
		if usage == nil {
			return nil
		}
		return hasUsage(&entity.Usage{
			InputTokens:       pickInt(usage, "input_tokens"),
			OutputTokens:      pickInt(usage, "output_tokens"),
			CachedInputTokens: pickInt(usage, "cached_input_tokens"),
		})
	}
	return nil
}
