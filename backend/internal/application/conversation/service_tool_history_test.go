package conversation

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type toolHistoryRepositoryStub struct {
	repository.ConversationRepository
	histories map[uint]string
	readUser  uint
}

func (r *toolHistoryRepositoryStub) SaveMessageToolHistory(_ context.Context, _, _, id uint, _, payload string) error {
	r.histories[id] = payload
	return nil
}
func (r *toolHistoryRepositoryStub) ListMessageToolHistories(_ context.Context, user uint, ids []uint) (map[uint]string, error) {
	r.readUser = user
	result := map[uint]string{}
	for _, id := range ids {
		result[id] = r.histories[id]
	}
	return result, nil
}

func TestToolHistoryRoundTripAndStatefulPrefix(t *testing.T) {
	r := &toolHistoryRepositoryStub{histories: map[uint]string{}}
	s := &Service{repo: r}
	call := llm.Message{Role: "assistant", Content: "looking", ReasoningContent: "reason", ToolCalls: []llm.ToolCall{{ToolCallID: "call-1", ToolName: "read_file", ArgumentsJSON: `{"file_id":"a"}`, ThoughtSignature: "signed"}}}
	result := llm.Message{Role: "tool", ToolResults: []llm.ToolResult{{ToolCallID: "call-1", ToolName: "read_file", OutputJSON: `{"content":"完整结果"}`}}}
	messages := []llm.Message{{Role: "system", Content: "system"}, {Role: "user", Content: "question"}, call, result}
	if err := s.persistMessageToolHistory(t.Context(), 1, 2, 3, "run", messages, nil, nil); err != nil {
		t.Fatal(err)
	}
	domain := []model.Message{{Role: "user", Content: "question"}, {ID: 3, ConversationID: 2, RunID: "run", Role: "assistant", Content: "answer"}, {Role: "user", Content: "next"}}
	replayed, err := s.replayMessageToolHistory(t.Context(), 1, 2, domain, historyMessagesFromDomain(domain, historyMessageOptions{}), true)
	if err != nil {
		t.Fatal(err)
	}
	want := []llm.Message{{Role: "user", Content: "question"}, call, result, {Role: "assistant", Content: "answer"}, {Role: "user", Content: "next"}}
	if !reflect.DeepEqual(replayed, want) {
		t.Fatalf("replay mismatch: %#v", replayed)
	}
	next := buildNextStatefulPrefixMessages(messages[1:], "question", "answer", "")
	if !reflect.DeepEqual(next, promptStatePrefixMessages(replayed)) {
		t.Fatal("Responses next-turn prefix loses tool transcript")
	}
	withoutReasoning, err := s.replayMessageToolHistory(t.Context(), 1, 2, domain, historyMessagesFromDomain(domain, historyMessageOptions{}), false)
	if err != nil || withoutReasoning[1].ReasoningContent != "" || withoutReasoning[1].ToolCalls[0].ThoughtSignature != "signed" {
		t.Fatal("reasoning policy or signature lost", err)
	}
	replayed[0].Content = strings.Repeat("old history ", 20000)
	trimmed, changed := trimToolFollowUpHistory(llm.GenerateInput{}, replayed, "custom", `{"contextWindow":4096,"maxOutputTokens":1024}`)
	if !changed || len(trimmed) != 1 || trimmed[0].Role != "user" {
		t.Fatal("must trim complete old turn, not half a tool pair")
	}
	for _, bad := range [][]llm.Message{{call}, {result}, {call, result, result}} {
		if validToolHistory(bad) {
			t.Fatal("accepted orphan or duplicate tool messages")
		}
	}
	if !validToolHistory([]llm.Message{call, result, call, result}) {
		t.Fatal("a provider may reuse a completed call ID in a later round")
	}
}

func TestRoutePromptReplaysToolsAcrossModelsInCurrentConversation(t *testing.T) {
	history := []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{{ToolCallID: "call-1", ToolName: "read_file", ArgumentsJSON: `{"file_id":"f"}`, ThoughtSignature: "signature"}}}, {Role: "tool", ToolResults: []llm.ToolResult{{ToolCallID: "call-1", ToolName: "read_file", OutputJSON: `{"content":"original result"}`}}}}
	raw, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}
	r := &toolHistoryRepositoryStub{histories: map[uint]string{3: string(raw), 99: string(raw)}}
	s := &Service{repo: r, cfg: config.NewRuntime(config.Config{})}
	domain := []model.Message{{ID: 99, ConversationID: 8, RunID: "reference", Role: "assistant", Content: "reference text"}, {ID: 2, ConversationID: 2, Role: "user", Content: "question"}, {ID: 3, ConversationID: 2, RunID: "run", Role: "assistant", Content: "answer"}, {ID: 4, ConversationID: 2, Role: "user", Content: "next"}}
	for _, route := range []channel.ResolvedRoute{
		{Protocol: llm.AdapterOpenAIChatCompletions, UpstreamModel: "deepseek-flash"},
		{Protocol: llm.AdapterOpenAIResponses, UpstreamModel: "gpt-5"},
		{Protocol: llm.AdapterAnthropicMessages, UpstreamModel: "claude-sonnet"},
		{Protocol: llm.AdapterGoogleGenerateContent, UpstreamModel: "gemini-2.5-pro"},
	} {
		plan, err := s.buildMessageRoutePrompt(t.Context(), &route, messageRoutePromptInput{UserID: 7, ConversationID: 2, UserContent: "next", DomainMessages: domain, SkipImageAttachments: true})
		if err != nil {
			t.Fatal(err)
		}
		calls, results := 0, 0
		for _, m := range plan.Messages {
			calls += len(m.ToolCalls)
			results += len(m.ToolResults)
			if len(m.ToolCalls) > 0 && !reflect.DeepEqual(m.ToolCalls, history[0].ToolCalls) {
				t.Fatal("changed original call")
			}
			if len(m.ToolResults) > 0 && !reflect.DeepEqual(m.ToolResults, history[1].ToolResults) {
				t.Fatal("changed original result")
			}
		}
		if calls != 1 || results != 1 || r.readUser != 7 {
			t.Fatalf("%s: calls=%d results=%d user=%d", route.UpstreamModel, calls, results, r.readUser)
		}
	}
}

func TestToolHistoryScrubsPreviouslySeenCredential(t *testing.T) {
	r := &toolHistoryRepositoryStub{histories: map[uint]string{}}
	s := &Service{repo: r}
	messages := []llm.Message{{Role: "user", Content: "q"}, {Role: "assistant", ToolCalls: []llm.ToolCall{{ToolCallID: "1", ArgumentsJSON: `{"value":"super-secret"}`}}}, {Role: "tool", ToolResults: []llm.ToolResult{{ToolCallID: "1", OutputJSON: `{"content":"super-secret"}`}}}}
	if err := s.persistMessageToolHistory(t.Context(), 1, 2, 3, "run", messages, []credentialWrite{{Value: "super-secret"}}, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.histories[3], "super-secret") || !json.Valid([]byte(r.histories[3])) {
		t.Fatal("unsafe snapshot", r.histories[3])
	}
}

func TestCachedOnlyInputDoesNotFallBackToPaidInputEstimate(t *testing.T) {
	a := &messageUsageAccumulator{}
	a.beginCall(llm.GenerateInput{Messages: []llm.Message{{Role: "user", Content: "cached prompt"}}})
	a.addObservedUsage(llm.Usage{CacheReadTokens: 100})
	a.finishCall(hasObservedInputUsage(a.usage()))
	if a.effectiveInputTokens(999) != 0 || a.interruptedInputTokens() != 0 {
		t.Fatalf("cached input billed twice: %+v", a)
	}
}
