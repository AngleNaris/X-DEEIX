package conversation

import (
	"context"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/mcp"
)

// llmGateway is the conversation generation boundary, not a concrete transport.
type llmGateway interface {
	Generate(context.Context, llm.RouteConfig, llm.GenerateInput) (*llm.GenerateOutput, error)
	GenerateStream(context.Context, llm.RouteConfig, llm.GenerateInput, func(llm.GenerateStreamEvent) error) (*llm.GenerateOutput, error)
	RetrieveOpenAIResponse(context.Context, llm.RouteConfig, string) (*llm.GenerateOutput, error)
	CancelOpenAIResponse(context.Context, llm.RouteConfig, string) (*llm.GenerateOutput, error)
}

// videoTaskGateway preserves custom deferred video retrieval independently of chat.
type videoTaskGateway interface {
	RetrieveVideoTask(context.Context, llm.RouteConfig, string) (*llm.VideoTaskRetrieval, error)
}

// mcpToolCaller 执行远端 MCP 工具调用。
type mcpToolCaller interface {
	CallTool(ctx context.Context, cfg mcp.CallConfig, input mcp.CallInput) (string, error)
}

