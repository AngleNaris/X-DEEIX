package conversation

import (
	"context"
	"encoding/json"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"go.uber.org/zap"
	"strings"
)

func (r *messageTraceRecorder) scrubCredentialAttempts(ctx context.Context, attempts []credentialWrite, successful []credentialWrite) {
	if !r.enabled() || len(attempts) == 0 {
		return
	}
	// Completed trace snapshots persist in background. Wait before the forced scrub so
	// an older plaintext snapshot cannot overwrite the sanitized row afterward.
	r.waitForPendingPersistence(context.WithoutCancel(ctx))
	for _, draft := range []*messageTraceDraft{r.process, r.tools, r.upstreamThink} {
		if draft == nil {
			continue
		}
		draft.title, _ = applyCredentialReplacements(draft.title, attempts, successful)
		draft.summary, _ = applyCredentialReplacements(draft.summary, attempts, successful)
		draft.contentMarkdown, _ = applyCredentialReplacements(draft.contentMarkdown, attempts, successful)
		draft.payload = scrubCredentialTracePayload(draft.payload, attempts, successful)
		r.persistDraftCtx(ctx, draft, true)
	}
	for index := range r.events {
		r.events[index].Title, _ = applyCredentialReplacements(r.events[index].Title, attempts, successful)
		r.events[index].Summary, _ = applyCredentialReplacements(r.events[index].Summary, attempts, successful)
		r.events[index].ContentMarkdown, _ = applyCredentialReplacements(r.events[index].ContentMarkdown, attempts, successful)
		r.events[index].PayloadJSON, _ = applyCredentialReplacementsToJSON(r.events[index].PayloadJSON, attempts, successful)
		r.persistTraceEventSnapshotRow(ctx, r.events[index])
	}
	r.upstreamThinkPendingReplace, _ = applyCredentialReplacements(r.upstreamThinkPendingReplace, attempts, successful)
	if pending, changed := applyCredentialReplacements(r.upstreamThinkPendingText.String(), attempts, successful); changed {
		r.upstreamThinkPendingText.Reset()
		_, _ = r.upstreamThinkPendingText.WriteString(pending)
	}
	r.upstreamThinkPendingReason = scrubCredentialTracePayload(r.upstreamThinkPendingReason, attempts, successful)
	scrubMessagePromptTraceCredentials(r.promptTrace, attempts, successful)
}

// Preserve the concrete payload type while scrubbing both legacy and typed traces.
func scrubCredentialTracePayload[T any](payload T, attempts []credentialWrite, successful []credentialWrite) T {
	raw, err := json.Marshal(payload)
	if err != nil {
		return payload
	}
	scrubbed, changed := applyCredentialReplacementsToJSON(string(raw), attempts, successful)
	if !changed {
		return payload
	}
	var result T
	if err := json.Unmarshal([]byte(scrubbed), &result); err != nil {
		return payload
	}
	return result
}

func scrubMessagePromptTraceCredentials(trace *model.MessagePromptTrace, attempts []credentialWrite, successful []credentialWrite) {
	if trace == nil {
		return
	}
	trace.PromptFingerprint, _ = applyCredentialReplacements(trace.PromptFingerprint, attempts, successful)
	trace.StatefulDisabledReason, _ = applyCredentialReplacements(trace.StatefulDisabledReason, attempts, successful)
	for index := range trace.Blocks {
		trace.Blocks[index].Title, _ = applyCredentialReplacements(trace.Blocks[index].Title, attempts, successful)
		for refIndex := range trace.Blocks[index].SourceRefs {
			trace.Blocks[index].SourceRefs[refIndex].SourceType, _ = applyCredentialReplacements(trace.Blocks[index].SourceRefs[refIndex].SourceType, attempts, successful)
			trace.Blocks[index].SourceRefs[refIndex].SourceID, _ = applyCredentialReplacements(trace.Blocks[index].SourceRefs[refIndex].SourceID, attempts, successful)
			trace.Blocks[index].SourceRefs[refIndex].Title, _ = applyCredentialReplacements(trace.Blocks[index].SourceRefs[refIndex].Title, attempts, successful)
		}
	}
}

// extractUpstreamThinkArtifacts keeps renderable artifacts in the assistant body, not the reasoning trace.
func (r *messageTraceRecorder) extractUpstreamThinkArtifacts() string {
	if !r.enabled() || r.upstreamThink == nil {
		return ""
	}
	artifacts, remaining := normalizeAssistantArtifactContent("", r.upstreamThink.contentMarkdown)
	if strings.TrimSpace(artifacts) == "" {
		return ""
	}
	r.upstreamThink.contentMarkdown = remaining
	r.upstreamThink.summary = summarizeThinkText(remaining)
	r.resetUpstreamThinkLiveBuffer()
	r.emitUpstreamThinkDelta(upstreamThinkLiveUpdate{
		kind:            messageTraceThinkKindContent,
		contentMarkdown: remaining,
		replaceContent:  true,
	})
	r.persistDraft(r.upstreamThink, true)
	return artifacts
}

func (r *messageTraceRecorder) persistTraceEventSnapshotRow(ctx context.Context, event model.MessageTraceEvent) {
	if !r.enabled() || r.ephemeral || r.service == nil || r.service.repo == nil {
		return
	}
	item := &model.MessageTraceEventRow{
		MessageID:       r.assistant.ID,
		ConversationID:  r.assistant.ConversationID,
		UserID:          r.assistant.UserID,
		RunID:           r.assistant.RunID,
		EventID:         event.EventID,
		EventType:       event.EventType,
		Phase:           event.Phase,
		Stage:           event.Stage,
		RoundID:         event.RoundID,
		ParentEventID:   event.ParentEventID,
		Status:          event.Status,
		Title:           event.Title,
		Summary:         truncateError(strings.TrimSpace(event.Summary), 255),
		ContentMarkdown: event.ContentMarkdown,
		PayloadJSON:     event.PayloadJSON,
		Seq:             event.Seq,
		StartedAt:       event.StartedAt,
		EndedAt:         event.EndedAt,
	}
	if err := r.service.repo.UpsertConversationMessageTraceEvent(ctx, item); err != nil && r.service.logger != nil {
		r.service.logger.Warn("scrub_conversation_message_trace_event_failed",
			zap.Uint("assistant_message_id", r.assistant.ID),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
	}
}
