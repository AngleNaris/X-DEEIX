package conversation

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"

var (
	ErrFileNotFound         = repository.ErrNotFound
	ErrStorageQuotaExceeded = repository.ErrStorageQuotaExceeded
)
