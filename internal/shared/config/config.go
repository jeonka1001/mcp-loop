// Package config는 config/agents.json 설정 파일을 로드한다.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"mcp-loop/internal/shared/apperr"
)

// AgentOverride는 어댑터 기본값 위에 덮어쓸 설정이다. 포인터 필드는 "미지정"과 "명시적 값"을 구분한다.
type AgentOverride struct {
	Enabled   *bool             `json:"enabled"`
	Command   string            `json:"command"`
	ExtraArgs []string          `json:"extraArgs"`
	Model     *string           `json:"model"`
	TimeoutMs *int              `json:"timeoutMs"`
	Env       map[string]string `json:"env"`
}

// Defaults는 에이전트 공통 기본값이다.
type Defaults struct {
	TimeoutMs      int `json:"timeoutMs"`
	MaxConcurrency int `json:"maxConcurrency"`
}

// Loop는 설정 파일 전체 구조다.
type Loop struct {
	Defaults Defaults                 `json:"defaults"`
	Agents   map[string]AgentOverride `json:"agents"`
}

func fallback() Loop {
	return Loop{
		Defaults: Defaults{TimeoutMs: 180_000, MaxConcurrency: 3},
		Agents:   map[string]AgentOverride{},
	}
}

// ResolvePath는 설정 파일 경로를 정한다: MCP_LOOP_CONFIG > 바이너리 기준 config/agents.json.
func ResolvePath() string {
	if fromEnv := os.Getenv("MCP_LOOP_CONFIG"); fromEnv != "" {
		return fromEnv
	}
	exe, err := os.Executable()
	if err != nil {
		return "config/agents.json"
	}
	dir := filepath.Dir(exe)
	for _, candidate := range []string{
		filepath.Join(dir, "config", "agents.json"),
		filepath.Join(dir, "..", "config", "agents.json"),
	} {
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
	}
	return filepath.Join(dir, "..", "config", "agents.json")
}

// Load는 설정 파일을 읽는다. 파일이 없으면 기본값을 쓰고, 그 외 읽기 실패나 형식이 깨졌으면 ConfigError를 반환한다.
func Load(path string) (Loop, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fallback(), nil
	}
	if err != nil {
		return Loop{}, apperr.Configf("설정 파일 읽기 실패 (%s): %v", path, err)
	}
	cfg := fallback()
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Loop{}, apperr.Configf("설정 파일 파싱 실패 (%s): %v", path, err)
	}
	return normalize(cfg), nil
}

func normalize(cfg Loop) Loop {
	if cfg.Defaults.TimeoutMs <= 0 {
		cfg.Defaults.TimeoutMs = 180_000
	}
	if cfg.Defaults.MaxConcurrency < 1 {
		cfg.Defaults.MaxConcurrency = 1
	}
	if cfg.Agents == nil {
		cfg.Agents = map[string]AgentOverride{}
	}
	return cfg
}
