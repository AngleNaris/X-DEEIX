package conversation

// TemporaryChatMessageRequest carries browser-owned history without persistent conversation records.
type TemporaryChatMessageRequest struct {
 SessionID string `json:"sessionID" binding:"required,max=64"`
 ClientRunID string `json:"clientRunID" binding:"required,max=64"`
 Model string `json:"model" binding:"required,max=128"`
 Options map[string]any `json:"options,omitempty"`
 SelectedToolIDs []uint `json:"selectedToolIDs,omitempty" binding:"max=128"`
 SkillIDs []uint `json:"skillIDs,omitempty" binding:"max=128"`
 KnowledgeBaseIDs []string `json:"knowledgeBaseIDs,omitempty" binding:"omitempty,max=8,dive,max=32"`
 HTMLVisualPrompt bool `json:"htmlVisualPrompt,omitempty"`
 Messages []TemporaryChatHistoryMessage `json:"messages" binding:"required,min=1,max=100,dive"`
 UIComponentIDs []uint `json:"uiComponentIDs,omitempty" binding:"max=32"`
}

// TemporaryChatHistoryMessage restricts client history to user and assistant turns.
type TemporaryChatHistoryMessage struct {
 Role string `json:"role" binding:"required,oneof=user assistant"`
 Content string `json:"content" binding:"max=200000"`
}
