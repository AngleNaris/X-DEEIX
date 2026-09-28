package contentmoderation

import (
	"context"
	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"testing"
)

func TestSyncRunPendingPreservesEphemeralBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		registered, ephemeral bool
		want                  string
	}{
		{"persistent", true, false, domaincm.ModerationStatePending},
		{"ephemeral", true, true, "unchanged"},
		{"unknown", false, false, "unchanged"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &coordinatorTestRepo{runState: "unchanged"}
			svc := &Service{repo: repo, coordinators: map[string]*RunCoordinator{}}
			if tc.registered {
				svc.coordinators["run_test"] = &RunCoordinator{meta: RunMeta{RunID: "run_test", Ephemeral: tc.ephemeral}}
			}
			svc.SyncRunPending(context.Background(), " run_test ")
			if repo.runState != tc.want {
				t.Fatalf("state = %q, want %q", repo.runState, tc.want)
			}
		})
	}
	var nilService *Service
	nilService.SyncRunPending(context.Background(), "run_test")
	(&Service{}).SyncRunPending(context.Background(), "run_test")
}
