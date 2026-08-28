// Command smoke는 mcp-loop 서버를 실제 MCP 클라이언트로 구동해 동작을 확인한다.
//
//	go run ./cmd/smoke                      핸드셰이크 + 툴 목록 + list_agents
//	go run ./cmd/smoke codex                위 + codex에 실제 질의
//	go run ./cmd/smoke codex,gemini "질문"   다중 에이전트 질의
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverBinary = "bin/mcp-loop"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "smoke 실패: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	session, err := connect(ctx)
	if err != nil {
		return err
	}
	defer session.Close()

	if err := showTools(ctx, session); err != nil {
		return err
	}
	if err := call(ctx, session, "list_agents", map[string]any{}); err != nil {
		return err
	}
	return askSelected(ctx, session)
}

func connect(ctx context.Context) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "0.0.1"}, nil)
	cmd := exec.Command(serverBinary)
	cmd.Stderr = os.Stderr
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("서버 연결 실패 (%s 빌드 여부 확인): %w", serverBinary, err)
	}
	fmt.Printf("=== initialize ===\n%+v\n", session.InitializeResult().ServerInfo)
	return session, nil
}

func showTools(ctx context.Context, session *mcp.ClientSession) error {
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("tools/list 실패: %w", err)
	}
	fmt.Println("\n=== tools/list ===")
	for _, tool := range result.Tools {
		fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
	}
	return nil
}

// askSelected는 인자로 지정된 에이전트에만 실제 질의를 보낸다.
func askSelected(ctx context.Context, session *mcp.ClientSession) error {
	agents := parseAgents()
	if len(agents) == 0 {
		return nil
	}
	prompt := "Reply with exactly: PONG"
	if len(os.Args) > 2 {
		prompt = os.Args[2]
	}
	if len(agents) == 1 {
		return call(ctx, session, "ask_agent", map[string]any{"agent": agents[0], "prompt": prompt})
	}
	return call(ctx, session, "ask_agents", map[string]any{"agents": agents, "prompt": prompt})
}

func parseAgents() []string {
	if len(os.Args) < 2 {
		return nil
	}
	var agents []string
	for _, part := range strings.Split(os.Args[1], ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			agents = append(agents, trimmed)
		}
	}
	return agents
}

func call(ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) error {
	encoded, _ := json.Marshal(args)
	fmt.Printf("\n=== %s %s ===\n", name, string(encoded))
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return fmt.Errorf("%s 호출 실패: %w", name, err)
	}
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			fmt.Println(text.Text)
		}
	}
	if result.IsError {
		fmt.Println("(isError=true)")
	}
	return nil
}
