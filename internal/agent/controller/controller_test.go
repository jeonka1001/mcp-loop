package controller_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-loop/internal/agent/adapter"
	"mcp-loop/internal/agent/controller"
	"mcp-loop/internal/agent/service"
	"mcp-loop/internal/shared/config"
)

// newSession은 인메모리 전송으로 서버-클라이언트를 연결한다. 실제 CLI는 호출하지 않는다.
func newSession(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	cfg := config.Loop{
		Defaults: config.Defaults{TimeoutMs: 180_000, MaxConcurrency: 3},
		Agents:   map[string]config.AgentOverride{},
	}
	registry := service.NewRegistry(cfg, []adapter.Adapter{adapter.Gemini{}, adapter.Codex{}, adapter.Claude{}})
	server := mcp.NewServer(&mcp.Implementation{Name: "mcp-loop", Version: "test"}, nil)
	controller.New(registry, service.NewQueryService(registry)).Register(server)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("서버 연결 실패: %v", err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).
		Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("클라이언트 연결 실패: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestListToolsExposesThreeTools(t *testing.T) {
	result, err := newSession(t).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list 실패: %v", err)
	}
	if len(result.Tools) != 3 {
		t.Fatalf("툴 3개를 기대했지만 %d개: %+v", len(result.Tools), result.Tools)
	}
}

func TestListAgentsReportsAllAdapters(t *testing.T) {
	text, isError := callTool(t, newSession(t), "list_agents", map[string]any{})
	if isError {
		t.Fatalf("list_agents가 실패했다: %s", text)
	}
	for _, id := range []string{"gemini", "codex", "claude"} {
		if !strings.Contains(text, id) {
			t.Errorf("list_agents 결과에 %q가 없다: %s", id, text)
		}
	}
}

// 선택되지 않은 에이전트가 호출되지 않도록, 잘못된 선택 인자는 모두 거부되어야 한다.
func TestSelectionArgumentsAreRejected(t *testing.T) {
	cases := []struct {
		name string
		tool string
		args map[string]any
	}{
		{"agent 누락", "ask_agent", map[string]any{"prompt": "hi"}},
		{"알 수 없는 agent", "ask_agent", map[string]any{"agent": "gpt5", "prompt": "hi"}},
		{"prompt 누락", "ask_agent", map[string]any{"agent": "codex"}},
		{"agents 빈 배열", "ask_agents", map[string]any{"agents": []string{}, "prompt": "hi"}},
		{"agents 누락", "ask_agents", map[string]any{"prompt": "hi"}},
		{"timeout_ms 범위 밖", "ask_agent", map[string]any{"agent": "codex", "prompt": "hi", "timeout_ms": 100}},
	}
	session := newSession(t)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assertRejected(t, session, testCase.tool, testCase.args)
		})
	}
}

// assertRejected는 스키마 거부(프로토콜 에러)와 도메인 거부(isError) 둘 다 통과로 본다.
func assertRejected(t *testing.T, session *mcp.ClientSession, tool string, args map[string]any) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Logf("스키마 거부: %v", err)
		return
	}
	if !result.IsError {
		t.Fatalf("거부를 기대했지만 성공했다: %+v", result)
	}
	t.Logf("도메인 거부: %s", firstText(result))
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s 호출 실패: %v", name, err)
	}
	return firstText(result), result.IsError
}

func firstText(result *mcp.CallToolResult) string {
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			return text.Text
		}
	}
	return ""
}
