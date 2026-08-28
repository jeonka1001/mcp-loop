// Package adapter는 각 CLI의 호출 규약(인자 생성 + 출력 파싱)을 캡슐화한다.
package adapter

import (
	"encoding/json"
	"strings"
	"time"

	"mcp-loop/internal/agent/entity"
	"mcp-loop/internal/shared/apperr"
	"mcp-loop/internal/shared/process"
)

// Defaults는 어댑터가 제공하는 기본 설정이다. 설정 파일이 이 값을 덮어쓸 수 있다.
type Defaults struct {
	Command   string
	ExtraArgs []string
	Model     string
	Timeout   time.Duration
	Env       map[string]string
}

// Parsed는 CLI 출력에서 추출한 최종 답변이다.
type Parsed struct {
	Text  string
	Usage *entity.Usage
}

// Adapter는 "구독 세션으로 CLI를 비대화형 실행한다"는 도메인 규칙만 담는다. 실행 자체는 process 패키지가 맡는다.
type Adapter interface {
	ID() string
	Label() string
	Defaults() Defaults
	// BlockedEnvKeys는 API 키 폴백을 막기 위해 제거할 환경변수다.
	BlockedEnvKeys() []string
	// AuthNote는 list_agents에 함께 노출할 인증/설정 안내다.
	AuthNote() string
	BuildArgs(cfg entity.Config, query entity.Query) []string
	// Parse는 답변 추출에 실패하면 apperr.ExecutionError를 반환한다.
	Parse(result process.RunResult) (Parsed, error)
}

// decodeObject는 노이즈가 섞인 출력에서 최상위 JSON 객체 하나를 복원한다.
func decodeObject(text string) map[string]any {
	trimmed := strings.TrimSpace(text)
	for offset := strings.IndexByte(trimmed, '{'); offset != -1; {
		if decoded := decodeAt(trimmed[offset:]); decoded != nil {
			return decoded
		}
		next := strings.IndexByte(trimmed[offset+1:], '{')
		if next == -1 {
			return nil
		}
		offset += next + 1
	}
	return nil
}

// decodeAt은 선두의 JSON 객체 하나만 읽고 뒤따르는 잡음은 무시한다.
func decodeAt(text string) map[string]any {
	var decoded map[string]any
	if err := json.NewDecoder(strings.NewReader(text)).Decode(&decoded); err != nil {
		return nil
	}
	return decoded
}

// decodeLines는 JSONL 스트림을 파싱한다. 파싱 불가한 줄(진행 로그 등)은 건너뛴다.
func decodeLines(text string) []map[string]any {
	var events []map[string]any
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "{") {
			continue
		}
		if decoded := decodeAt(trimmed); decoded != nil {
			events = append(events, decoded)
		}
	}
	return events
}

// pickInt는 JSON 숫자(float64)를 int 포인터로 꺼낸다. 없으면 nil.
func pickInt(source map[string]any, keys ...string) *int {
	for _, key := range keys {
		if value, ok := source[key].(float64); ok {
			converted := int(value)
			return &converted
		}
	}
	return nil
}

func pickString(source map[string]any, key string) string {
	value, _ := source[key].(string)
	return value
}

func nestedMap(source map[string]any, key string) map[string]any {
	nested, _ := source[key].(map[string]any)
	return nested
}

// hasUsage는 수집된 사용량 중 하나라도 값이 있는지 확인한다.
func hasUsage(usage *entity.Usage) *entity.Usage {
	if usage.InputTokens == nil && usage.OutputTokens == nil && usage.CachedInputTokens == nil {
		return nil
	}
	return usage
}

// failureDetail은 진단용으로 남길 원본 출력을 고른다.
// CLI마다 오류를 내보내는 스트림이 달라서 출처를 함께 기록한다.
func failureDetail(result process.RunResult) apperr.Detail {
	if text := strings.TrimSpace(result.Stderr); text != "" {
		return apperr.Detail{Text: text, Source: "stderr"}
	}
	return apperr.Detail{Text: strings.TrimSpace(result.Stdout), Source: "stdout"}
}

// withModel은 모델이 지정된 경우에만 --model 인자를 붙인다.
func withModel(args []string, cfg entity.Config, query entity.Query) []string {
	model := query.Model
	if model == "" {
		model = cfg.Model
	}
	if model == "" {
		return args
	}
	return append(args, "--model", model)
}
