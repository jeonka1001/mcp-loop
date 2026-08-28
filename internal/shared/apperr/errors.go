// Package apperr는 계층 간에 의미를 유지해야 하는 도메인 에러를 정의한다.
package apperr

import "fmt"

// ValidationError는 도메인 규칙 위반(잘못된 입력)을 나타낸다.
// MCP 클라이언트에게 그대로 노출해도 되는 메시지를 담는다.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

// Validationf는 서식 문자열로 ValidationError를 만든다.
func Validationf(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

// ExecutionError는 에이전트 CLI 실행/파싱 실패를 나타낸다.
type ExecutionError struct {
	AgentID string
	Msg     string
	// Detail은 진단용 원본 출력이고, DetailSource는 그것이 나온 스트림 이름이다.
	Detail       string
	DetailSource string
}

func (e *ExecutionError) Error() string { return e.Msg }

// Executionf는 서식 문자열로 ExecutionError를 만든다.
func Executionf(agentID string, detail Detail, format string, args ...any) error {
	return &ExecutionError{
		AgentID:      agentID,
		Msg:          fmt.Sprintf(format, args...),
		Detail:       detail.Text,
		DetailSource: detail.Source,
	}
}

// Detail은 실패 진단에 쓸 원본 출력과 그 출처다.
type Detail struct {
	Text   string
	Source string
}

// ConfigError는 설정 파일 로드/파싱 실패를 나타낸다.
type ConfigError struct {
	Msg string
}

func (e *ConfigError) Error() string { return e.Msg }

// Configf는 서식 문자열로 ConfigError를 만든다.
func Configf(format string, args ...any) error {
	return &ConfigError{Msg: fmt.Sprintf(format, args...)}
}
