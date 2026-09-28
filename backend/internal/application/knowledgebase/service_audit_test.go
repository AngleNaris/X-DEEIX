package knowledgebase

import (
	"context"
	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	"testing"
)

type auditCapture struct {
	input appaudit.WriteInput
	calls int
}

func (w *auditCapture) Write(_ context.Context, input appaudit.WriteInput) {
	w.input = input
	w.calls++
}

func TestRecordAuditRetainsAllFields(t *testing.T) {
	svc := NewService(nil)
	svc.RecordAudit(context.Background(), AuditInput{})
	writer := &auditCapture{}
	svc.SetAuditWriter(writer)
	svc.RecordAudit(context.Background(), AuditInput{UserID: 7, RequestID: " req ", Action: " update ", ResourceID: " kb ", ClientIP: " 127.0.0.1 ", UserAgent: " test ", Detail: "safe detail"})
	want := appaudit.WriteInput{ActorUserID: 7, RequestID: "req", Action: "update", Resource: "knowledge_bases", ResourceID: "kb", IP: "127.0.0.1", UserAgent: "test", Detail: "safe detail"}
	if writer.calls != 1 || writer.input != want {
		t.Fatalf("lost audit fields: calls=%d input=%+v", writer.calls, writer.input)
	}
}

var _ auditWriter = (*appaudit.Service)(nil)
