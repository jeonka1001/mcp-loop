// Command mcp-loop는 구독 세션으로 로그인된 CLI 에이전트에 질의를 중계하는 MCP 서버다.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-loop/internal/agent/adapter"
	"mcp-loop/internal/agent/controller"
	"mcp-loop/internal/agent/service"
	"mcp-loop/internal/shared/config"
)

const (
	serverName    = "mcp-loop"
	serverVersion = "0.1.0"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-loop 시작 실패: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(config.ResolvePath())
	if err != nil {
		return err
	}
	registry := service.NewRegistry(cfg, []adapter.Adapter{adapter.Codex{}, adapter.Claude{}, adapter.Antigravity{}})
	ctrl := controller.New(registry, service.NewQueryService(registry))

	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)
	ctrl.Register(server)
	return server.Run(context.Background(), &mcp.StdioTransport{})
}
