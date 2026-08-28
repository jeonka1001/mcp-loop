// Package service는 에이전트 도메인의 비즈니스 로직을 담당한다.
package service

import (
	"time"

	"mcp-loop/internal/agent/adapter"
	"mcp-loop/internal/agent/entity"
	"mcp-loop/internal/shared/apperr"
	"mcp-loop/internal/shared/config"
	"mcp-loop/internal/shared/process"
)

// Registry는 어댑터 기본값과 설정 파일 override를 합쳐 사용 가능한 에이전트를 관리한다.
type Registry struct {
	cfg      config.Loop
	adapters []adapter.Adapter
	byID     map[string]adapter.Adapter
}

// NewRegistry는 어댑터 목록을 등록 순서대로 유지하는 레지스트리를 만든다.
func NewRegistry(cfg config.Loop, adapters []adapter.Adapter) *Registry {
	byID := make(map[string]adapter.Adapter, len(adapters))
	for _, item := range adapters {
		byID[item.ID()] = item
	}
	return &Registry{cfg: cfg, adapters: adapters, byID: byID}
}

// MaxConcurrency는 ask_agents 동시 실행 상한이다.
func (r *Registry) MaxConcurrency() int { return r.cfg.Defaults.MaxConcurrency }

// IDs는 등록된 에이전트 id를 등록 순서대로 반환한다.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.adapters))
	for _, item := range r.adapters {
		ids = append(ids, item.ID())
	}
	return ids
}

// List는 모든 에이전트의 설정/설치 상태를 반환한다.
func (r *Registry) List() []entity.Availability {
	list := make([]entity.Availability, 0, len(r.adapters))
	for _, item := range r.adapters {
		list = append(list, r.describe(item))
	}
	return list
}

// Require는 활성화되고 설치된 에이전트만 반환한다. 그 외에는 ValidationError.
func (r *Registry) Require(id string) (adapter.Adapter, entity.Config, error) {
	item, found := r.byID[id]
	if !found {
		return nil, entity.Config{}, apperr.Validationf("알 수 없는 에이전트 '%s'. 사용 가능: %v", id, r.IDs())
	}
	cfg := r.resolveConfig(item)
	if !cfg.Enabled {
		return nil, entity.Config{}, apperr.Validationf("에이전트 '%s'는 설정에서 비활성화되어 있습니다.", id)
	}
	if _, ok := process.LookPath(cfg.Command); !ok {
		return nil, entity.Config{}, apperr.Validationf("에이전트 '%s'의 CLI '%s'를 PATH에서 찾을 수 없습니다. %s", id, cfg.Command, item.AuthNote())
	}
	return item, cfg, nil
}

func (r *Registry) describe(item adapter.Adapter) entity.Availability {
	cfg := r.resolveConfig(item)
	path, installed := process.LookPath(cfg.Command)
	return entity.Availability{
		AgentID:      item.ID(),
		Label:        item.Label(),
		Enabled:      cfg.Enabled,
		Command:      cfg.Command,
		Installed:    installed,
		ResolvedPath: path,
		DefaultModel: cfg.Model,
		TimeoutMs:    cfg.Timeout.Milliseconds(),
		Note:         item.AuthNote(),
	}
}

func (r *Registry) resolveConfig(item adapter.Adapter) entity.Config {
	defaults := item.Defaults()
	override := r.cfg.Agents[item.ID()]
	return entity.Config{
		ID:        item.ID(),
		Label:     item.Label(),
		Enabled:   boolOr(override.Enabled, true),
		Command:   stringOr(override.Command, defaults.Command),
		ExtraArgs: sliceOr(override.ExtraArgs, defaults.ExtraArgs),
		Model:     stringOr(derefString(override.Model), defaults.Model),
		Timeout:   r.resolveTimeout(override.TimeoutMs, defaults.Timeout),
		Env:       mergeEnv(defaults.Env, override.Env),
	}
}

func (r *Registry) resolveTimeout(overrideMs *int, adapterDefault time.Duration) time.Duration {
	if overrideMs != nil && *overrideMs > 0 {
		return time.Duration(*overrideMs) * time.Millisecond
	}
	if adapterDefault > 0 {
		return adapterDefault
	}
	return time.Duration(r.cfg.Defaults.TimeoutMs) * time.Millisecond
}

func mergeEnv(base, override map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(override))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func boolOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func sliceOr(value, fallback []string) []string {
	if len(value) == 0 {
		return fallback
	}
	return value
}
