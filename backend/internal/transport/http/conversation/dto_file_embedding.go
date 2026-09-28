package conversation

import appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"

type GetFileProcessingStatusesRequest struct {
	FileIDs []string `json:"fileIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}

type SubmitFileEmbeddingsRequest struct {
	FileIDs []string `json:"fileIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}

type FileEmbeddingSkipResponse struct {
	FileID string `json:"fileID"`
	Reason string `json:"reason"`
}

type FileEmbeddingSubmissionResponse struct {
	SubmittedFileIDs []string                    `json:"submittedFileIDs"`
	Skipped          []FileEmbeddingSkipResponse `json:"skipped"`
}

func toFileEmbeddingSubmissionResponse(result appembedding.TargetedSubmissionResult) FileEmbeddingSubmissionResponse {
	skipped := make([]FileEmbeddingSkipResponse, 0, len(result.Skipped))
	for _, item := range result.Skipped {
		skipped = append(skipped, FileEmbeddingSkipResponse{FileID: item.FileID, Reason: item.Reason})
	}
	return FileEmbeddingSubmissionResponse{SubmittedFileIDs: result.SubmittedFileIDs, Skipped: skipped}
}

type FileEmbeddingSubmissionResponseDoc struct {
	ErrorMsg string                          `json:"errorMsg"`
	Data     FileEmbeddingSubmissionResponse `json:"data"`
}
