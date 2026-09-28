package conversation

import (
	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

// ModerationBlockedBilledReason preserves billing for upstream usage already incurred.
// Withdrawing content does not undo paid usage; free and self-billed calls have no reservation.
func ModerationBlockedBilledReason(result *SendMessageResult, authorization *domainbilling.UsageAuthorization) string {
	if !result.IsModerationBlocked() || !result.Billable {
		return ""
	}
	return paidUsageBilledReason(authorization)
}

func liveModerationBlockedBilledReason(direction string, authorization *domainbilling.UsageAuthorization) string {
	if direction != appcm.DirectionOutput {
		return ""
	}
	return paidUsageBilledReason(authorization)
}

func paidUsageBilledReason(authorization *domainbilling.UsageAuthorization) string {
	if authorization == nil || authorization.Reservation == nil {
		return ""
	}
	return appbilling.BilledReasonModerationBlockedUpstreamUsage
}

func moderationLiveEmitter(onEvent func(string, map[string]any) error, authorization *domainbilling.UsageAuthorization) appcm.LiveEmitter {
	return func(eventType string, payload map[string]any) {
		if eventType == "moderation_blocked" && payload != nil {
			direction, _ := payload["direction"].(string)
			if reason := liveModerationBlockedBilledReason(direction, authorization); reason != "" {
				payload["billedReason"] = reason
			}
		}
		_ = onEvent(eventType, payload)
	}
}
