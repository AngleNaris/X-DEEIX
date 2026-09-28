package conversation

import model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"

// ConversationToolCallDetailResponse retains bounded-payload omission metadata.
type ConversationToolCallDetailResponse struct {
 RunID string `json:"runID"`
 ToolCallID string `json:"toolCallID"`
 ToolName string `json:"toolName"`
 Status string `json:"status"`
 OutputJSON string `json:"outputJSON"`
 OutputSizeBytes int64 `json:"outputSizeBytes"`
 OutputOmitted bool `json:"outputOmitted"`
 ErrorJSON string `json:"errorJSON"`
 ErrorSizeBytes int64 `json:"errorSizeBytes"`
 ErrorOmitted bool `json:"errorOmitted"`
}

func toConversationToolCallDetailResponse(item *model.ToolCallDetail) ConversationToolCallDetailResponse {
 return ConversationToolCallDetailResponse{
  RunID: item.RunID, ToolCallID: item.ToolCallID, ToolName: item.ToolName, Status: item.Status,
  OutputJSON: item.OutputJSON, OutputSizeBytes: item.OutputSizeBytes, OutputOmitted: item.OutputOmitted,
  ErrorJSON: item.ErrorJSON, ErrorSizeBytes: item.ErrorSizeBytes, ErrorOmitted: item.ErrorOmitted,
 }
}

// ConversationToolCallDetailResponseDoc describes the detail endpoint envelope.
type ConversationToolCallDetailResponseDoc struct {
 ErrorMsg string `json:"errorMsg"`
 Data ConversationToolCallDetailResponse `json:"data"`
}
