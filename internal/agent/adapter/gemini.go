package adapter

import (
	"time"

	"mcp-loop/internal/agent/entity"
	"mcp-loop/internal/shared/apperr"
	"mcp-loop/internal/shared/process"
)

const geminiID = "gemini"

// Gemini는 gemini CLI 어댑터다. ~/.gemini/oauth_creds.json 의 Google OAuth 세션을 그대로 사용한다.
type Gemini struct{}

func (Gemini) ID() string    { return geminiID }
func (Gemini) Label() string { return "Gemini CLI" }

func (Gemini) Defaults() Defaults {
	return Defaults{Command: "gemini", Timeout: 3 * time.Minute, Env: map[string]string{}}
}

func (Gemini) BlockedEnvKeys() []string {
	return []string{
		"GEMINI_API_KEY",
		"GOOGLE_API_KEY",
		"GOOGLE_GENAI_USE_VERTEXAI",
		"GOOGLE_GENAI_USE_GCA",
		"GOOGLE_APPLICATION_CREDENTIALS",
	}
}

func (Gemini) AuthNote() string {
	return "gemini CLI의 OAuth 로그인 세션을 사용합니다. 계정이 Code Assist 프로젝트를 요구하면 config/agents.json의 agents.gemini.env에 GOOGLE_CLOUD_PROJECT를 지정하세요."
}

func (Gemini) BuildArgs(cfg entity.Config, query entity.Query) []string {
	args := withModel([]string{"--output-format", "json"}, cfg, query)
	return append(args, cfg.ExtraArgs...)
}

func (Gemini) Parse(result process.RunResult) (Parsed, error) {
	// gemini CLI는 구조화된 오류(JSON)를 stderr로 내보내므로 stdout이 비면 stderr도 확인한다.
	body := decodeObject(result.Stdout)
	if body == nil {
		body = decodeObject(result.Stderr)
	}
	if body == nil {
		return Parsed{}, apperr.Executionf(geminiID, failureDetail(result), "Gemini CLI 출력에서 JSON 응답을 찾지 못했습니다.")
	}
	if err := geminiError(body, result); err != nil {
		return Parsed{}, err
	}
	text := geminiText(body)
	if text == "" {
		return Parsed{}, apperr.Executionf(geminiID, failureDetail(result), "Gemini CLI 응답에 response 필드가 없습니다.")
	}
	return Parsed{Text: text, Usage: geminiUsage(body)}, nil
}

func geminiError(body map[string]any, result process.RunResult) error {
	failure := nestedMap(body, "error")
	if failure == nil {
		return nil
	}
	message := pickString(failure, "message")
	if message == "" {
		message = "알 수 없는 오류"
	}
	return apperr.Executionf(geminiID, failureDetail(result), "Gemini CLI 오류: %s", message)
}

func geminiText(body map[string]any) string {
	if response := pickString(body, "response"); response != "" {
		return response
	}
	return pickString(body, "text")
}

func geminiUsage(body map[string]any) *entity.Usage {
	stats := nestedMap(body, "stats")
	if stats == nil {
		return nil
	}
	tokens := nestedMap(stats, "tokens")
	if tokens == nil {
		tokens = stats
	}
	return hasUsage(&entity.Usage{
		InputTokens:       pickInt(tokens, "prompt", "promptTokenCount"),
		OutputTokens:      pickInt(tokens, "candidates", "candidatesTokenCount"),
		CachedInputTokens: pickInt(tokens, "cached", "cachedContentTokenCount"),
	})
}
