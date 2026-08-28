// Package entity는 에이전트 도메인의 데이터 모델과 핵심 검증 규칙을 정의한다.
package entity

import (
	"time"

	"mcp-loop/internal/shared/apperr"
)

const (
	MinTimeout      = 5 * time.Second
	MaxTimeout      = 15 * time.Minute
	MaxPromptLength = 200_000
)

// Config는 어댑터 기본값과 설정 파일 override를 합친 최종 에이전트 설정이다.
type Config struct {
	Enabled   bool
	Command   string
	ExtraArgs []string
	Model     string
	Timeout   time.Duration
	Env       map[string]string
}

// Query는 검증을 통과한 1회 질의 요청이다.
type Query struct {
	Prompt  string
	Model   string
	Timeout time.Duration
	Cwd     string
}

// Usage는 CLI가 보고한 토큰 사용량이다. 보고하지 않는 항목은 nil이다.
type Usage struct {
	InputTokens       *int `json:"inputTokens,omitempty"`
	OutputTokens      *int `json:"outputTokens,omitempty"`
	CachedInputTokens *int `json:"cachedInputTokens,omitempty"`
}

// Answer는 1회 질의 결과다. 실패해도 예외가 아니라 이 형태로 수집한다.
type Answer struct {
	AgentID    string `json:"agentId"`
	OK         bool   `json:"ok"`
	Text       string `json:"text"`
	Usage      *Usage `json:"usage,omitempty"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// Availability는 list_agents가 반환하는 에이전트 상태다.
type Availability struct {
	AgentID      string `json:"agentId"`
	Label        string `json:"label"`
	Enabled      bool   `json:"enabled"`
	Command      string `json:"command"`
	Installed    bool   `json:"installed"`
	ResolvedPath string `json:"resolvedPath,omitempty"`
	DefaultModel string `json:"defaultModel,omitempty"`
	TimeoutMs    int64  `json:"timeoutMs"`
	Note         string `json:"note,omitempty"`
}

// NewQuery는 원시 툴 입력을 검증된 Query로 변환한다.
func NewQuery(prompt, model string, timeoutMs *int, cwd string) (Query, error) {
	if err := validatePrompt(prompt); err != nil {
		return Query{}, err
	}
	timeout, err := parseTimeout(timeoutMs)
	if err != nil {
		return Query{}, err
	}
	return Query{Prompt: prompt, Model: model, Timeout: timeout, Cwd: cwd}, nil
}

func validatePrompt(prompt string) error {
	if len(prompt) == 0 {
		return apperr.Validationf("prompt는 비어 있지 않은 문자열이어야 합니다.")
	}
	if len(prompt) > MaxPromptLength {
		return apperr.Validationf("prompt가 너무 깁니다. 최대 %d자.", MaxPromptLength)
	}
	return nil
}

func parseTimeout(timeoutMs *int) (time.Duration, error) {
	if timeoutMs == nil {
		return 0, nil
	}
	timeout := time.Duration(*timeoutMs) * time.Millisecond
	if timeout < MinTimeout || timeout > MaxTimeout {
		return 0, apperr.Validationf("timeout_ms는 %d~%d 범위여야 합니다.",
			MinTimeout.Milliseconds(), MaxTimeout.Milliseconds())
	}
	return timeout, nil
}

// RequireAgentID는 단일 에이전트 지정 인자를 검증한다.
func RequireAgentID(agent string) (string, error) {
	if agent == "" {
		return "", apperr.Validationf("agent를 반드시 지정해야 합니다. list_agents로 사용 가능한 id를 확인하세요.")
	}
	return agent, nil
}

// RequireAgentIDs는 다중 에이전트 인자를 검증하고 중복을 제거한다.
// 빈 배열을 허용하지 않으므로 "전체 자동 호출" 경로는 존재하지 않는다.
func RequireAgentIDs(agents []string) ([]string, error) {
	if len(agents) == 0 {
		return nil, apperr.Validationf("agents 배열에 최소 1개 이상의 에이전트 id를 지정해야 합니다. 자동 전체 호출은 지원하지 않습니다.")
	}
	seen := make(map[string]bool, len(agents))
	unique := make([]string, 0, len(agents))
	for i, agent := range agents {
		if agent == "" {
			return nil, apperr.Validationf("agents[%d]는 비어 있지 않은 문자열이어야 합니다.", i)
		}
		if !seen[agent] {
			seen[agent] = true
			unique = append(unique, agent)
		}
	}
	return unique, nil
}
