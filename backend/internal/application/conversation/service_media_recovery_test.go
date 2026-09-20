package conversation

import (
	"context"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"testing"
)

type recoveryResolver struct {
	routeResolver
	personal, platform int
}

func (r *recoveryResolver) BuildRouteForUserModel(_ context.Context, owner, id, upstream uint, protocol, name string) (*channel.ResolvedRoute, error) {
	r.personal++
	return &channel.ResolvedRoute{UpstreamOwnerUserID: &owner, UpstreamModelID: id, UpstreamID: upstream, Protocol: protocol, UpstreamModel: name}, nil
}
func (r *recoveryResolver) BuildRouteForUpstream(_ context.Context, id uint, protocol, name string) (*channel.ResolvedRoute, error) {
	r.platform++
	return &channel.ResolvedRoute{UpstreamID: id}, nil
}
func TestMediaRecoveryRouteScope(t *testing.T) {
	for _, tc := range []struct {
		binding            string
		personal, platform int
		invalid            bool
	}{{"user-model-3", 1, 0, false}, {"platform-binding", 0, 1, false}, {"", 0, 1, false}, {"user-model-0", 0, 0, true}, {"user-model-03", 0, 0, true}, {"user-model--1", 0, 0, true}, {"user-model-x", 0, 0, true}} {
		t.Run(tc.binding, func(t *testing.T) {
			r := &recoveryResolver{}
			s := &Service{routeResolver: r}
			route, err := s.buildMediaRecoveryRoute(t.Context(), &model.Run{UserID: 7, UpstreamID: 9, RoutedBindingCode: tc.binding, ProviderProtocol: "openai_video_generations", UpstreamModelName: "original"})
			if (err != nil) != tc.invalid || r.personal != tc.personal || r.platform != tc.platform {
				t.Fatalf("err=%v personal=%d platform=%d", err, r.personal, r.platform)
			}
			if tc.personal == 1 && ((route.UpstreamOwnerUserID == nil || *route.UpstreamOwnerUserID != 7) || route.UpstreamModelID != 3 || route.UpstreamID != 9 || route.UpstreamModel != "original") {
				t.Fatal("run identity lost")
			}
		})
	}
}
