package conversation

// ConversationRunStatusResponse is the bounded batch status projection.
type ConversationRunStatusResponse struct {
	RunID string `json:"runID"`
	Status string `json:"status"`
}
