package conversation

import (
	"context"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"strings"
)

// activeModelCatalogResolver validates defaults against the user's visible catalog,
// not transient route health or circuit breaker state.
type activeModelCatalogResolver interface {
	ListActiveModels(context.Context, uint) ([]channel.ModelView, error)
}

func (s *Service) isAvailableConversationProjectDefaultModel(ctx context.Context, userID uint, platformModelName string) (bool, error) {
	resolver, ok := s.routeResolver.(activeModelCatalogResolver)
	if !ok {
		return false, nil
	}
	models, err := resolver.ListActiveModels(ctx, userID)
	if err != nil {
		return false, err
	}
	name := strings.TrimSpace(platformModelName)
	for _, item := range models {
		if strings.TrimSpace(item.PlatformModelName) == name && channel.ModelSupportsTask(item.KindsJSON, channel.TaskTypeChat) {
			return true, nil
		}
	}
	return false, nil
}
