package channel

import "context"

// BuildRouteForUserModel rejects recovery of retired personal routes without platform fallback.
func (s *Service) BuildRouteForUserModel(ctx context.Context, userID, userModelID, upstreamID uint, protocol, upstreamModel string) (*ResolvedRoute, error) {
	return nil, ErrModelAccessDenied
}
