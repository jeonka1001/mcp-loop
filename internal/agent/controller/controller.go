// Package controller는 MCP 툴 진입점이다. 입력 검증은 entity에, 실행은 service에 위임한다.
package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-loop/internal/agent/entity"
	"mcp-loop/internal/agent/service"
)

// Controller는 MCP 서버에 툴을 등록하고 요청을 서비스로 넘긴다.
type Controller struct {
	registry *service.Registry
	query    *service.QueryService
}

// New는 컨트롤러를 만든다.
func New(registry *service.Registry, query *service.QueryService) *Controller {
	return &Controller{registry: registry, query: query}
}

type listAgentsInput struct{}

type askAgentInput struct {
	Agent     string `json:"agent"`
	Prompt    string `json:"prompt"`
	Model     string `json:"model,omitempty"`
	TimeoutMs *int   `json:"timeout_ms,omitempty"`
	Cwd       string `json:"cwd,omitempty"`
}

type askAgentsInput struct {
	Agents    []string `json:"agents"`
	Prompt    string   `json:"prompt"`
	Model     string   `json:"model,omitempty"`
	TimeoutMs *int     `json:"timeout_ms,omitempty"`
	Cwd       string   `json:"cwd,omitempty"`
}

// Register는 세 개의 툴을 MCP 서버에 등록한다.
func (c *Controller) Register(server *mcp.Server) {
	ids := c.registry.IDs()
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_agents",
		Description: "사용 가능한 에이전트(gemini/codex/claude)의 설치 상태, 활성화 여부, 기본 모델, 인증 안내를 조회한다.",
		InputSchema: &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{}},
	}, c.listAgents)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ask_agent",
		Description: "지정한 단일 에이전트 하나에만 질의한다. agent는 필수이며, 지정하지 않으면 호출되지 않는다.",
		InputSchema: askAgentSchema(ids),
	}, c.askAgent)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ask_agents",
		Description: "명시적으로 선택한 여러 에이전트에 같은 질의를 병렬 전달한다. agents 배열은 필수이며 비어 있을 수 없다. 전체 자동 호출은 지원하지 않는다.",
		InputSchema: askAgentsSchema(ids),
	}, c.askAgents)
}

func (c *Controller) listAgents(ctx context.Context, _ *mcp.CallToolRequest, _ listAgentsInput) (*mcp.CallToolResult, any, error) {
	encoded, err := json.MarshalIndent(c.registry.List(), "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("에이전트 목록 직렬화 실패: %v", err)), nil, nil
	}
	return textResult(string(encoded), false), nil, nil
}

func (c *Controller) askAgent(ctx context.Context, _ *mcp.CallToolRequest, in askAgentInput) (*mcp.CallToolResult, any, error) {
	agentID, err := entity.RequireAgentID(in.Agent)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}
	query, err := entity.NewQuery(in.Prompt, in.Model, in.TimeoutMs, in.Cwd)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}
	answer := c.query.Ask(ctx, agentID, query)
	return textResult(renderAnswer(answer), !answer.OK), nil, nil
}

func (c *Controller) askAgents(ctx context.Context, _ *mcp.CallToolRequest, in askAgentsInput) (*mcp.CallToolResult, any, error) {
	agentIDs, err := entity.RequireAgentIDs(in.Agents)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}
	query, err := entity.NewQuery(in.Prompt, in.Model, in.TimeoutMs, in.Cwd)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}
	answers := c.query.AskMany(ctx, agentIDs, query)
	return textResult(renderAnswers(answers), allFailed(answers)), nil, nil
}

func textResult(text string, isError bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: isError}
}

func errorResult(text string) *mcp.CallToolResult { return textResult(text, true) }

func allFailed(answers []entity.Answer) bool {
	for _, answer := range answers {
		if answer.OK {
			return false
		}
	}
	return true
}

func renderAnswer(answer entity.Answer) string {
	if !answer.OK {
		return fmt.Sprintf("[%s] 실패 (%dms)\n%s", answer.AgentID, answer.DurationMs, answer.Error)
	}
	return fmt.Sprintf("[%s] %s\n\n%s", answer.AgentID, formatUsage(answer), answer.Text)
}

func renderAnswers(answers []entity.Answer) string {
	states := make([]string, 0, len(answers))
	blocks := make([]string, 0, len(answers)+1)
	for _, answer := range answers {
		state := "fail"
		if answer.OK {
			state = "ok"
		}
		states = append(states, answer.AgentID+"="+state)
		blocks = append(blocks, strings.Repeat("=", 60)+"\n"+renderAnswer(answer))
	}
	return "요약: " + strings.Join(states, ", ") + "\n" + strings.Join(blocks, "\n")
}

func formatUsage(answer entity.Answer) string {
	parts := []string{fmt.Sprintf("%dms", answer.DurationMs)}
	if answer.Usage != nil && answer.Usage.InputTokens != nil {
		parts = append(parts, fmt.Sprintf("in=%d", *answer.Usage.InputTokens))
	}
	if answer.Usage != nil && answer.Usage.OutputTokens != nil {
		parts = append(parts, fmt.Sprintf("out=%d", *answer.Usage.OutputTokens))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}
