package channel

import (
	"errors"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"testing"
)

func TestPersonalChannelsCannotBeReenabled(t *testing.T) {
	s := NewService(config.Config{UserUpstreamEnabled: true, UserUpstreamBillingMode: "statistics_only"}, nil, nil, nil, nil)
	for _, input := range []ResolveRouteInput{
		{ModelScope: "user", UserID: 7, UserModelID: 3},
		{ModelScope: " USER ", UserID: 7},
		{ModelScope: "platform", UserID: 7, UserModelID: 3},
	} {
		if err := s.ValidateModelRouteReference(t.Context(), input); !errors.Is(err, ErrModelAccessDenied) {
			t.Fatalf("personal model accepted: %v", err)
		}
	}
	if _, err := s.BuildRouteForUserModel(t.Context(), 7, 3, 9, "openai_video_generations", "original-model"); !errors.Is(err, ErrModelAccessDenied) {
		t.Fatalf("personal recovery accepted: %v", err)
	}
}
