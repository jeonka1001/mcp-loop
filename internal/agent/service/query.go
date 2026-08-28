package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"mcp-loop/internal/agent/adapter"
	"mcp-loop/internal/agent/entity"
	"mcp-loop/internal/shared/apperr"
	"mcp-loop/internal/shared/process"
)

const maxDetailTail = 2_000

// QueryService는 선택된 에이전트에만 질의를 전달한다. 자동 전체 호출은 이 계층에도 존재하지 않는다.
type QueryService struct {
	registry *Registry
}

// NewQueryService는 레지스트리에 묶인 질의 서비스를 만든다.
func NewQueryService(registry *Registry) *QueryService {
	return &QueryService{registry: registry}
}

// Ask는 단일 에이전트에 질의한다. 실행 실패는 에러가 아니라 OK=false 결과로 반환한다.
func (s *QueryService) Ask(ctx context.Context, agentID string, query entity.Query) entity.Answer {
	item, cfg, err := s.registry.Require(agentID)
	if err != nil {
		return entity.Answer{AgentID: agentID, Error: err.Error()}
	}
	return execute(ctx, item, cfg, query)
}

// AskMany는 클라이언트가 명시적으로 고른 에이전트들만 동시성 제한 하에 병렬 호출한다.
func (s *QueryService) AskMany(ctx context.Context, agentIDs []string, query entity.Query) []entity.Answer {
	answers := make([]entity.Answer, len(agentIDs))
	limit := min(s.registry.MaxConcurrency(), len(agentIDs))
	slots := make(chan struct{}, max(limit, 1))

	var wg sync.WaitGroup
	for index, agentID := range agentIDs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			answers[index] = s.Ask(ctx, agentID, query)
		}()
	}
	wg.Wait()
	return answers
}

func execute(ctx context.Context, item adapter.Adapter, cfg entity.Config, query entity.Query) entity.Answer {
	result := process.Run(ctx, runOptions(item, cfg, query))
	durationMs := result.Duration.Milliseconds()

	if result.TimedOut {
		return fail(item.ID(), durationMs, fmt.Sprintf("타임아웃(%dms) 초과로 중단했습니다.", timeoutOf(cfg, query).Milliseconds()))
	}
	if result.SpawnErr != nil {
		return fail(item.ID(), durationMs, fmt.Sprintf("CLI 실행 실패: %v", result.SpawnErr))
	}
	parsed, err := item.Parse(result)
	if err != nil {
		return fail(item.ID(), durationMs, describeFailure(result, err))
	}
	return entity.Answer{AgentID: item.ID(), OK: true, Text: parsed.Text, Usage: parsed.Usage, DurationMs: durationMs}
}

func runOptions(item adapter.Adapter, cfg entity.Config, query entity.Query) process.RunOptions {
	return process.RunOptions{
		Command:        cfg.Command,
		Args:           item.BuildArgs(cfg, query),
		Stdin:          query.Prompt,
		Dir:            query.Cwd,
		Timeout:        timeoutOf(cfg, query),
		BlockedEnvKeys: item.BlockedEnvKeys(),
		ExtraEnv:       cfg.Env,
	}
}

// timeoutOf는 호출별 타임아웃이 있으면 그것을, 없으면 에이전트 기본값을 쓴다.
func timeoutOf(cfg entity.Config, query entity.Query) time.Duration {
	if query.Timeout > 0 {
		return query.Timeout
	}
	return cfg.Timeout
}

// describeFailure는 실패 메시지에 종료 코드와 CLI 출력 꼬리를 덧붙인다. 오류는 보통 끝부분에 있다.
func describeFailure(result process.RunResult, err error) string {
	message := err.Error()
	if result.ExitCode >= 0 {
		message = fmt.Sprintf("%s (exit=%d)", message, result.ExitCode)
	}
	var execErr *apperr.ExecutionError
	if !errors.As(err, &execErr) {
		return message
	}
	detail := strings.TrimSpace(execErr.Detail)
	if detail == "" {
		return message
	}
	if len(detail) > maxDetailTail {
		detail = detail[len(detail)-maxDetailTail:]
	}
	return fmt.Sprintf("%s\n--- %s (tail) ---\n%s", message, execErr.DetailSource, detail)
}

func fail(agentID string, durationMs int64, message string) entity.Answer {
	return entity.Answer{AgentID: agentID, DurationMs: durationMs, Error: message}
}
