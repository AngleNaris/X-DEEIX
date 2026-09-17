package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type conversationReadRepository struct {
	repository.ConversationRepository
	body string
	read bool
}

func (r *conversationReadRepository) GetConversationByUser(_ context.Context, id, user uint) (*model.Conversation, error) {
	if id != 2 || user != 7 {
		return nil, errors.New("denied")
	}
	return &model.Conversation{ID: id, UserID: user, Title: "test"}, nil
}
func (r *conversationReadRepository) GetMessageByID(_ context.Context, conversation, id uint) (*model.Message, error) {
	r.read = true
	if conversation != 2 || id != 3 {
		return nil, errors.New("not found")
	}
	return &model.Message{ID: 3, Role: "assistant", Content: r.body}, nil
}
func (r *conversationReadRepository) ListMessagesBeforeID(_ context.Context, _ uint, before uint, limit int) ([]model.Message, int64, error) {
	r.read = true
	end := 65
	if before > 0 {
		end = int(before) - 1
	}
	items := []model.Message{}
	for i := max(1, end-limit+1); i <= end; i++ {
		items = append(items, model.Message{ID: uint(i), Role: "user", Content: r.body})
	}
	return items, 65, nil
}

func TestPlatformReadConversationAllPagesAndLongMessage(t *testing.T) {
	r := &conversationReadRepository{body: strings.Repeat("星🌟中文", 1800)}
	s := &Service{repo: r}
	seen := map[uint]bool{}
	var before uint
	for {
		raw, err := s.platformReadConversation(t.Context(), platformToolCallContext{UserID: 7, Arguments: []byte(fmt.Sprintf(`{"conversation_id":2,"before_id":%d}`, before))})
		if err != nil {
			t.Fatal(err)
		}
		var page struct {
			Messages []struct {
				ID        uint   `json:"message_id"`
				Content   string `json:"content"`
				Next      int    `json:"next_offset"`
				Truncated bool   `json:"truncated"`
			} `json:"messages"`
			More bool `json:"has_more"`
			Next uint `json:"next_before_id"`
		}
		if err := json.Unmarshal([]byte(raw), &page); err != nil {
			t.Fatal(err)
		}
		if before == 0 && (len(page.Messages) != 20 || page.Messages[19].ID != 65) {
			t.Fatal("not latest page")
		}
		for _, m := range page.Messages {
			if seen[m.ID] || !utf8.ValidString(m.Content) || !m.Truncated || m.Next != len(m.Content) {
				t.Fatalf("invalid preview: %+v", m)
			}
			seen[m.ID] = true
		}
		if !page.More {
			break
		}
		if page.Next >= before && before > 0 {
			t.Fatal("cursor stalled")
		}
		before = page.Next
	}
	if len(seen) != 65 {
		t.Fatalf("read %d messages", len(seen))
	}
	var full strings.Builder
	for offset := 0; ; {
		raw, err := s.platformReadConversation(t.Context(), platformToolCallContext{UserID: 7, Arguments: []byte(fmt.Sprintf(`{"conversation_id":2,"message_id":3,"offset":%d,"max_bytes":127}`, offset))})
		if err != nil {
			t.Fatal(err)
		}
		var page struct {
			Content string `json:"content"`
			Next    int    `json:"next_offset"`
			More    bool   `json:"has_more"`
		}
		if err := json.Unmarshal([]byte(raw), &page); err != nil {
			t.Fatal(err)
		}
		full.WriteString(page.Content)
		if !page.More {
			break
		}
		if page.Next <= offset {
			t.Fatal("cursor stalled")
		}
		offset = page.Next
	}
	if full.String() != r.body {
		t.Fatal("long message lost bytes")
	}
	r.read = false
	if _, err := s.platformReadConversation(t.Context(), platformToolCallContext{UserID: 8, Arguments: []byte(`{"conversation_id":2,"message_id":3}`)}); err == nil || r.read {
		t.Fatal("ownership not enforced")
	}
}

func TestKnowledgeBaseReadPagesUseVisibleFilePermission(t *testing.T) {
	stub := &knowledgeBaseToolStub{readText: strings.Repeat("星🌟", 100)}
	s := &Service{knowledgeBaseTools: stub}
	var full strings.Builder
	for offset := 0; ; {
		raw, err := s.platformReadKnowledgeBaseContent(t.Context(), platformToolCallContext{UserID: 7, Arguments: []byte(fmt.Sprintf(`{"knowledge_base_id":"builtin","content_id":"doc","offset":%d,"max_bytes":8}`, offset))})
		if err != nil {
			t.Fatal(err)
		}
		var page struct {
			Content string `json:"content"`
			Next    int    `json:"next_offset"`
			More    bool   `json:"has_more"`
		}
		if err := json.Unmarshal([]byte(raw), &page); err != nil {
			t.Fatal(err)
		}
		full.WriteString(page.Content)
		if !page.More {
			break
		}
		if page.Next <= offset {
			t.Fatal("cursor stalled")
		}
		offset = page.Next
	}
	if full.String() != stub.readText || stub.lastUserID != 7 || stub.lastKnowledgeBaseID != "builtin" {
		t.Fatal("read or authorization identity mismatch")
	}
	stub.readErr = errors.New("denied")
	if _, err := s.platformReadKnowledgeBaseContent(t.Context(), platformToolCallContext{UserID: 7, Arguments: []byte(`{"knowledge_base_id":"private","content_id":"doc"}`)}); !errors.Is(err, stub.readErr) {
		t.Fatal("authorization error bypassed")
	}
	if _, _, err := platformTextPage("星🌟", 1, 4); err == nil {
		t.Fatal("accepted invalid byte cursor")
	}
}

func TestPlatformToolOrderAndPagedBudgetRetry(t *testing.T) {
	s := &Service{platformToolsSettings: &fakePlatformSettingsReader{values: map[string]string{platformToolsKeyEnabled: "true", platformToolsKeyWriteEnabled: "true"}}}
	var previous []byte
	for i := 0; i < 20; i++ {
		runtime := selectedToolRuntime{}
		if err := s.appendPlatformToolRuntime(t.Context(), &runtime); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(runtime.definitions)
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && !reflect.DeepEqual(previous, raw) {
			t.Fatal("tools reordered")
		}
		previous = raw
	}
	raw, _ := marshalPlatformResult(map[string]interface{}{"file_id": "f", "content": strings.Repeat("中文", 1000), "offset": 123, "next_offset": 6123, "has_more": true})
	budgeted := budgetToolOutputForModel(raw, 200)
	var retry struct {
		Exceeded bool                   `json:"budget_exceeded"`
		Args     map[string]interface{} `json:"retry_same_tool_with"`
	}
	if err := json.Unmarshal([]byte(budgeted), &retry); err != nil || !retry.Exceeded || retry.Args["offset"] != float64(123) || retry.Args["file_id"] != "f" {
		t.Fatal("paging cursor lost:", budgeted, err)
	}
}
