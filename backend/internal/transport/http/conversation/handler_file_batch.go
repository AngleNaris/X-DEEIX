package conversation

import (
	"errors"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
)

// GetFileProcessingStatuses godoc
// @Summary 批量查询文件处理状态
// @Description 一次查询当前用户多个文件的处理状态
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Accept json
// @Param request body GetFileProcessingStatusesRequest true "文件ID，最多100个"
// @Success 200 {array} FileProcessingStatusResponse
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /files/processing/statuses [post]
func (h *Handler) GetFileProcessingStatuses(c *gin.Context) {
	var req GetFileProcessingStatusesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	result, err := h.processing.GetFileProcessingStatuses(
		c.Request.Context(),
		middleware.MustUserID(c),
		req.FileIDs,
	)
	if err != nil {
		response.InternalError(c)
		return
	}
	statuses := make([]FileProcessingStatusResponse, 0, len(result))
	for i := range result {
		statuses = append(statuses, toFileProcessingStatusResponse(&result[i]))
	}
	response.Success(c, statuses)
}

// SubmitFileEmbeddings godoc
// @Summary 批量提交指定文件向量化
// @Description 为当前用户已完成文本提取的文件提交向量化任务，最多100个；重复提交会幂等跳过
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Accept json
// @Param request body SubmitFileEmbeddingsRequest true "文件ID，最多100个"
// @Success 200 {object} FileEmbeddingSubmissionResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Failure 503 {object} ErrorDoc
// @Router /files/embeddings [post]
func (h *Handler) SubmitFileEmbeddings(c *gin.Context) {
	var req SubmitFileEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.processing.SubmitFileEmbeddings(c.Request.Context(), middleware.MustUserID(c), req.FileIDs)
	if err != nil {
		switch {
		case errors.Is(err, appembedding.ErrTooManyTargetedFiles):
			response.ErrorWithCode(c, http.StatusBadRequest, "embedding.too_many_files")
		case errors.Is(err, appembedding.ErrEmbeddingServiceNotConfigured):
			response.ErrorWithCode(c, http.StatusServiceUnavailable, "embedding.service_not_configured")
		case errors.Is(err, appembedding.ErrEmbeddingServiceUnavailable):
			response.ErrorWithCode(c, http.StatusServiceUnavailable, "embedding.service_unavailable")
		default:
			response.ErrorWithCode(c, http.StatusInternalServerError, "embedding.submit_failed")
		}
		return
	}
	h.recordAudit(c, "submit_file_embeddings", "file", "", map[string]any{
		"requested_file_ids": req.FileIDs,
		"submitted_file_ids": result.SubmittedFileIDs,
		"skipped_count":      len(result.Skipped),
	})
	response.Success(c, toFileEmbeddingSubmissionResponse(result))
}
