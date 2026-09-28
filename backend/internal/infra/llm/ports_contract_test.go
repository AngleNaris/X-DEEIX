package llm

import (
	"context"
	"errors"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// Compile-time checks exercise the application/transport boundary, unlike isolated package tests.
var _ interface {
	Generate(context.Context, portllm.RouteConfig, portllm.GenerateInput) (*portllm.GenerateOutput, error)
	GenerateStream(context.Context, portllm.RouteConfig, portllm.GenerateInput, func(portllm.GenerateStreamEvent) error) (*portllm.GenerateOutput, error)
	ListModels(context.Context, portllm.RouteConfig) ([]portllm.ModelItem, error)
} = (*Client)(nil)

func TestPortErrorIdentityAndAcceptedRequest(t *testing.T) {
	if !errors.Is(ErrUnsupportedAdapter, portllm.ErrUnsupportedAdapter) || !errors.Is(ErrUnsupportedStream, portllm.ErrUnsupportedStream) {
		t.Fatal("transport and application must share sentinel errors")
	}
	cause := &portllm.UpstreamError{StatusCode: 503, Message: "unavailable"}
	accepted := MarkRequestAccepted(cause)
	if !portllm.RequestWasAccepted(accepted) || !RequestWasAccepted(portllm.MarkRequestAccepted(cause)) {
		t.Fatal("request acceptance must be visible on both sides of the boundary")
	}
	var transportError *UpstreamError
	if !errors.As(accepted, &transportError) || transportError != cause {
		t.Fatal("upstream error identity must survive wrapping")
	}
	if MarkRequestAccepted(accepted) != accepted {
		t.Fatal("acceptance wrapping must be idempotent")
	}
}
