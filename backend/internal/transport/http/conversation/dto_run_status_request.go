package conversation

// GetConversationRunStatusesRequest bounds the batch of user-scoped run IDs.
type GetConversationRunStatusesRequest struct {
	RunIDs []string `json:"runIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}
