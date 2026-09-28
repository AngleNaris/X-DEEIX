package conversation

import (
	"errors"
	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
)

// mapRouteResolutionError retains access-control and upstream availability semantics.
func mapRouteResolutionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, appchannel.ErrModelAccessDenied):
		return ErrModelAccessDenied
	case errors.Is(err, appchannel.ErrRouteNotFound), errors.Is(err, appchannel.ErrModelNotFound):
		return ErrModelRouteNotConfigured
	case errors.Is(err, appchannel.ErrAllRoutesUnavailable), errors.Is(err, appchannel.ErrAllRoutesRateLimited):
		return wrapUpstreamRequestError(err)
	default:
		return err
	}
}
