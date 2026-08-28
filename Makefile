BINARY := bin/mcp-loop

.PHONY: build test vet smoke clean

build:
	go build -o $(BINARY) ./cmd/mcp-loop

test:
	go test ./...

vet:
	go vet ./...

# make smoke                    핸드셰이크 + 툴 목록 + list_agents
# make smoke AGENTS=codex       codex에 실제 질의
# make smoke AGENTS=codex,gemini PROMPT="질문"
smoke: build
	go run ./cmd/smoke $(AGENTS) $(if $(PROMPT),"$(PROMPT)",)

clean:
	rm -rf bin
