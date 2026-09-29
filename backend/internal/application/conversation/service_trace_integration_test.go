package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	persistencemodels "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	persistenceconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type assistantEditTraceRepository struct {
	repository.ConversationRepository
	message model.Message
}

func (r *assistantEditTraceRepository) GetMessageByPublicIDForUser(
	context.Context,
	uint,
	string,
) (*model.Message, error) {
	item := r.message
	return &item, nil
}

func (r *assistantEditTraceRepository) UpdateMessageContent(
	_ context.Context,
	_ uint,
	_ string,
	content string,
	editedAt time.Time,
) (*model.Message, error) {
	r.message.Content = content
	r.message.EditedAt = &editedAt
	item := r.message
	return &item, nil
}

func (r *assistantEditTraceRepository) GetUserMessageFeedbackMap(
	context.Context,
	uint,
	[]uint,
) (map[uint]string, error) {
	return map[uint]string{}, nil
}

func (r *assistantEditTraceRepository) GetMessageFeedbackCounts(
	context.Context,
	[]uint,
) (map[uint]map[string]int64, error) {
	return map[uint]map[string]int64{}, nil
}

func (r *assistantEditTraceRepository) ListConversationMessageTracesByMessageIDs(
	context.Context,
	[]uint,
) ([]model.MessageTrace, error) {
	return []model.MessageTrace{{
		MessageID:       r.message.ID,
		TraceType:       messageTraceTypeProcess,
		Title:           "Processing complete",
		ContentMarkdown: "Retained processing details",
		Status:          messageTraceStatusCompleted,
	}}, nil
}

func (r *assistantEditTraceRepository) ListConversationMessageTraceEventsByMessageIDs(
	context.Context,
	[]uint,
) ([]model.MessageTraceEventRow, error) {
	return []model.MessageTraceEventRow{{
		MessageID: r.message.ID,
		EventID:   "event_tool_1",
		EventType: "tool",
		Title:     "Tool complete",
		Status:    messageTraceStatusCompleted,
		Seq:       1,
	}}, nil
}
func (r *assistantEditTraceRepository) ListConversationToolCallsByMessageIDs(
	context.Context,
	[]uint,
) ([]model.ToolCall, error) {
	return nil, nil
}

func TestAssistantEditResponseRetainsProcessTrace(t *testing.T) {
	repo := &assistantEditTraceRepository{
		message: model.Message{
			ID:             41,
			ConversationID: 17,
			UserID:         9,
			PublicID:       "message_assistant_edit",
			Role:           "assistant",
			Status:         "success",
			Content:        "before",
		},
	}
	service := &Service{
		cfg: config.NewRuntime(config.Config{
			ProcessTraceEnabled: true,
		}),
		repo: repo,
	}

	updated, err := service.UpdateMessageContent(
		context.Background(),
		repo.message.UserID,
		repo.message.PublicID,
		"after",
	)
	if err != nil {
		t.Fatalf("edit assistant message: %v", err)
	}
	if updated.Content != "after" || updated.EditedAt == nil {
		t.Fatalf("unexpected edited message: %#v", updated)
	}
	if updated.ProcessTrace == nil || updated.ProcessTrace.Process == nil {
		t.Fatalf("edited response lost process trace: %#v", updated.ProcessTrace)
	}
	if len(updated.ProcessTrace.Events) != 1 || updated.ProcessTrace.Events[0].EventID != "event_tool_1" {
		t.Fatalf("edited response lost execution events: %#v", updated.ProcessTrace)
	}
}

func TestCanceledTraceSettlementPersistsCompleteReasoningForReload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:trace_cancel_settlement?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&persistencemodels.ChatRunEvent{}); err != nil {
		t.Fatalf("migrate trace table: %v", err)
	}

	repo := persistenceconversation.NewRepo(db)
	cfg := config.Config{
		ProcessTraceEnabled: true,
	}
	service := &Service{cfg: config.NewRuntime(cfg), repo: repo}
	assistant := &model.Message{
		ID:             41,
		ConversationID: 17,
		UserID:         9,
		RunID:          "run_cancel_settlement",
		Role:           "assistant",
	}

	generationCtx, cancelGeneration := context.WithCancel(context.Background())
	recorder := newMessageTraceRecorder(service, generationCtx, assistant, nil)
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "嗯", nil)
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "，继续分析并保留终止前的完整思考", nil)
	cancelGeneration()
	if generationCtx.Err() == nil {
		t.Fatal("expected generation context to be canceled")
	}

	recorder.failWithContext(context.Background(), ErrMessageGenerationCanceled)

	reloaded := []model.Message{{ID: assistant.ID, Role: "assistant"}}
	reloadService := &Service{cfg: config.NewRuntime(cfg), repo: repo}
	if err := reloadService.hydrateMessageProcessTraces(context.Background(), reloaded); err != nil {
		t.Fatalf("hydrate persisted trace: %v", err)
	}
	trace := reloaded[0].ProcessTrace
	if trace == nil || trace.UpstreamThink == nil {
		t.Fatalf("expected persisted upstream reasoning after reload, got %#v", trace)
	}
	if got, want := trace.UpstreamThink.ContentMarkdown, "嗯，继续分析并保留终止前的完整思考"; got != want {
		t.Fatalf("reloaded reasoning = %q, want %q", got, want)
	}
	if trace.UpstreamThink.Status != messageTraceStatusError {
		t.Fatalf("reloaded reasoning status = %q, want %q", trace.UpstreamThink.Status, messageTraceStatusError)
	}
}

func TestPlatformApprovalTerminalStateSurvivesServiceReload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:trace_approval_reload?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&persistencemodels.ChatRunEvent{}); err != nil {
		t.Fatalf("migrate trace table: %v", err)
	}

	repo := persistenceconversation.NewRepo(db)
	const pendingOutput = `{"status":"pending_approval","approval_id":"approval-reload","tool":"save_memory"}`
	_, _, tracePayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "call-reload",
		ToolName:   "save_memory",
		Status:     "success",
		OutputJSON: pendingOutput,
	}})
	payloadJSON, err := json.Marshal(tracePayload)
	if err != nil {
		t.Fatalf("marshal trace payload: %v", err)
	}
	if err := repo.UpsertConversationMessageTrace(context.Background(), &model.MessageTrace{
		MessageID:       71,
		ConversationID:  37,
		UserID:          19,
		RunID:           "run-approval-reload",
		TraceType:       messageTraceTypeTools,
		Status:          messageTraceStatusCompleted,
		ContentMarkdown: "pending",
		PayloadJSON:     string(payloadJSON),
	}); err != nil {
		t.Fatalf("persist trace: %v", err)
	}
	toolRow := model.ToolCall{
		MessageID:      71,
		ConversationID: 37,
		UserID:         19,
		RunID:          "run-approval-reload",
		ToolCallID:     "call-reload",
		ToolName:       "save_memory",
		Status:         "success",
		OutputJSON:     `{"approval_id":"approval-reload","status":"approved","tool":"save_memory"}`,
	}
	if err := repo.CreateConversationToolCall(context.Background(), &toolRow); err != nil {
		t.Fatalf("persist tool call: %v", err)
	}

	cfg := config.Config{ProcessTraceEnabled: true, ProcessTraceVisibleToUser: true}
	reloaded := []model.Message{{ID: 71, Role: "assistant"}}
	if err := (&Service{cfg: config.NewRuntime(cfg), repo: repo}).hydrateMessageProcessTraces(context.Background(), reloaded); err != nil {
		t.Fatalf("hydrate persisted trace: %v", err)
	}
	if reloaded[0].ProcessTrace == nil || reloaded[0].ProcessTrace.Tools == nil {
		t.Fatalf("missing reloaded tools trace: %#v", reloaded[0].ProcessTrace)
	}
	var hydrated struct {
		ToolCalls []struct {
			OutputDetail string `json:"output_detail"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(reloaded[0].ProcessTrace.Tools.PayloadJSON), &hydrated); err != nil {
		t.Fatalf("decode hydrated tools trace: %v", err)
	}
	if len(hydrated.ToolCalls) != 1 || !strings.Contains(hydrated.ToolCalls[0].OutputDetail, `"status":"approved"`) ||
		strings.Contains(hydrated.ToolCalls[0].OutputDetail, "pending_approval") {
		t.Fatalf("terminal approval was not reconciled after reload: %+v", hydrated.ToolCalls)
	}
}

func TestScrubCredentialAttemptsRewritesPersistedTraceEvents(t *testing.T) {
	const (
		secret = "trace-secret-value"
		ref    = "{{secret_ref:11111111-1111-1111-1111-111111111111}}"
	)
	db, err := gorm.Open(sqlite.Open("file:trace_credential_scrub?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&persistencemodels.ChatRunEvent{}); err != nil {
		t.Fatalf("migrate trace table: %v", err)
	}

	repo := persistenceconversation.NewRepo(db)
	cfg := config.Config{
		ProcessTraceEnabled:         true,
		ProcessTraceVisibleToUser:   true,
		ProcessTracePersistInflight: true,
	}
	service := &Service{cfg: config.NewRuntime(cfg), repo: repo}
	assistant := &model.Message{
		ID:             51,
		ConversationID: 27,
		UserID:         13,
		RunID:          "run_credential_scrub",
		Role:           "assistant",
	}
	recorder := newMessageTraceRecorder(service, context.Background(), assistant, nil)
	recorder.appendToolSection(
		"credential attempt "+secret,
		"credential attempt "+ref,
		&tracePayload{ToolCalls: []traceToolCall{{InputDetail: secret, OutputDetail: ref}}},
		messageTraceStatusCompleted,
	)
	recorder.scrubCredentialAttempts(context.Background(), []credentialWrite{{
		Name:  "deploy-key",
		Value: secret,
		Ref:   ref,
	}}, nil)

	rows, err := repo.ListConversationMessageTraceEventsByMessageIDs(context.Background(), []uint{assistant.ID})
	if err != nil {
		t.Fatalf("reload trace events: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected persisted trace events")
	}
	for _, row := range rows {
		serialized := row.Title + row.Summary + row.ContentMarkdown + row.PayloadJSON
		if strings.Contains(serialized, secret) || strings.Contains(serialized, ref) {
			t.Fatalf("persisted trace event retained credential material: %s", serialized)
		}
	}
}

func TestUpdateMessageContentAllowsUserAndAssistantOnly(t *testing.T) {
	cases := []struct {
		role    string
		wantErr error
	}{
		{role: "user"},
		{role: "assistant"},
		{role: "system", wantErr: ErrMessageEditTargetInvalid},
		{role: "tool", wantErr: ErrMessageEditTargetInvalid},
	}
	for _, tc := range cases {
		repo := &assistantEditTraceRepository{
			message: model.Message{ID: 1, ConversationID: 1, UserID: 9, PublicID: "message_" + tc.role, Role: tc.role, Status: "success", Content: "before"},
		}
		service := &Service{cfg: config.NewRuntime(config.Config{}), repo: repo}
		updated, err := service.UpdateMessageContent(context.Background(), 9, repo.message.PublicID, "after")
		if tc.wantErr != nil {
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("%s: got %v, want %v", tc.role, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", tc.role, err)
		}
		if updated.Content != "after" || updated.EditedAt == nil {
			t.Fatalf("%s: unexpected result %#v", tc.role, updated)
		}
	}
}
