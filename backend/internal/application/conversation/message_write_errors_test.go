package conversation

import (
	"errors"
	"fmt"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"testing"
)

func TestMessageWriteErrorMappingPreservesContract(t *testing.T) {
	for _, tc := range []struct{ source, target error }{
		{repository.ErrMessageParentDeleted, ErrMessageParentDeleted},
		{repository.ErrMessageDeleteStateInvalid, ErrMessageDeleteStateInvalid},
		{repository.ErrMessageDeleteRootInvalid, ErrMessageDeleteRootInvalid},
	} {
		if got := mapMessageWriteError(fmt.Errorf("storage: %w", tc.source)); got != tc.target {
			t.Errorf("map(%v) = %v, want %v", tc.source, got, tc.target)
		}
	}
	unrelated := errors.New("storage offline")
	if mapMessageWriteError(unrelated) != unrelated || mapMessageWriteError(nil) != nil {
		t.Fatal("unrelated errors and nil must pass through unchanged")
	}
}
