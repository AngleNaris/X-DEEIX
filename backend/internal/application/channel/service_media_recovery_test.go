package channel

import (
	"context"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"testing"
)

type recoveryRouteRepo struct {
	repository.ChannelRepository
	repository.UserModelRepository
	model    domainchannel.UserModel
	upstream domainchannel.Upstream
}

func (r *recoveryRouteRepo) GetUserModelByID(_ context.Context, owner, id uint) (*domainchannel.UserModel, error) {
	if owner != r.model.OwnerUserID || id != r.model.ID {
		return nil, repository.ErrNotFound
	}
	return &r.model, nil
}
func (r *recoveryRouteRepo) GetUserUpstreamByID(_ context.Context, owner, id uint) (*domainchannel.Upstream, error) {
	if (r.upstream.OwnerUserID == nil || owner != *r.upstream.OwnerUserID) || id != r.upstream.ID {
		return nil, repository.ErrNotFound
	}
	return &r.upstream, nil
}
func TestPersonalMediaRecoveryRoute(t *testing.T) {
	const key = "test-data-encryption-key-32-bytes"
	encrypted, err := encryptAPIKeys(key, `{"keys":[{"key":"local-fixture","status":"active"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	owner := uint(7)
	r := &recoveryRouteRepo{model: domainchannel.UserModel{ID: 3, OwnerUserID: 7, UpstreamID: 9, Status: "active", HeadersJSON: `{"X-Model":"yes"}`}, upstream: domainchannel.Upstream{ID: 9, OwnerUserID: &owner, Status: "active", BaseURL: "https://example.com/v1", APIKeysEnc: encrypted, HeadersJSON: `{"X-Upstream":"yes"}`}}
	s := NewService(config.Config{DataEncryptionKey: key}, r, nil, nil, nil)
	route, err := s.BuildRouteForUserModel(t.Context(), 7, 3, 9, "openai_video_generations", "original-model")
	if err != nil {
		t.Fatal(err)
	}
	if route.UpstreamID != 9 || (route.UpstreamOwnerUserID == nil || *route.UpstreamOwnerUserID != 7) || route.UpstreamModel != "original-model" || route.APIKey != "local-fixture" || route.Protocol != "openai_video_generations" {
		t.Fatalf("incorrect recovery route: %+v", route)
	}
	if route.HeadersJSON != `{"X-Model":"yes","X-Upstream":"yes"}` {
		t.Fatalf("headers lost: %s", route.HeadersJSON)
	}
	for _, tc := range []struct {
		name                   string
		owner, model, upstream uint
	}{{"other owner", 8, 3, 9}, {"wrong model", 7, 4, 9}, {"moved upstream", 7, 3, 10}, {"missing upstream", 7, 3, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.BuildRouteForUserModel(t.Context(), tc.owner, tc.model, tc.upstream, "openai_video_generations", "original-model"); err == nil {
				t.Fatal("unsafe recovery accepted")
			}
		})
	}
	r.upstream.Status = "disabled"
	if _, err := s.BuildRouteForUserModel(t.Context(), 7, 3, 9, "openai_video_generations", "original-model"); err == nil {
		t.Fatal("disabled upstream accepted")
	}
}
