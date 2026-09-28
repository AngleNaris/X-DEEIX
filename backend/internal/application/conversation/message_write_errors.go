package conversation

import (
	"errors"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// mapMessageWriteError preserves the application error contract for concurrent message mutations.
func mapMessageWriteError(err error) error {
	switch {
	case errors.Is(err, repository.ErrMessageParentDeleted):
		return ErrMessageParentDeleted
	case errors.Is(err, repository.ErrMessageDeleteStateInvalid):
		return ErrMessageDeleteStateInvalid
	case errors.Is(err, repository.ErrMessageDeleteRootInvalid):
		return ErrMessageDeleteRootInvalid
	default:
		return err
	}
}
