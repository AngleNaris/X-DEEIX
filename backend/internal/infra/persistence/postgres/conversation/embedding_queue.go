package conversation

import (
	"context"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"strings"
)

// QueueFileEmbedding atomically registers a job owned by the requesting user.
func (r *Repo) QueueFileEmbedding(ctx context.Context, userID uint, fileID string, embeddingSignature string) (bool, error) {
	fileID = strings.TrimSpace(fileID)
	embeddingSignature = strings.TrimSpace(embeddingSignature)
	if fileID == "" || embeddingSignature == "" {
		return false, repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Model(&models.FileObject{}).
		Where("user_id = ? AND file_id = ? AND status = ?", userID, fileID, "active").
		Where("NOT (embed_signature = ? AND embed_status IN ?)", embeddingSignature, []string{"queued", "processing", "ready"}).
		Updates(map[string]any{"embed_status": "queued", "embed_signature": embeddingSignature, "embed_error": ""})
	return result.RowsAffected > 0, dberror.Translate(result.Error)
}
