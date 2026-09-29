package repository

import (
	"context"
	"time"
)

// ProviderAuthTransaction is the short-lived server-side state for an OAuth
// provider authorization started by a public client.
type ProviderAuthTransaction struct {
	ProviderSlug         string `json:"providerSlug"`
	ClientID             string `json:"clientID"`
	ClientRedirectURI    string `json:"clientRedirectURI"`
	ClientState          string `json:"clientState"`
	ClientCodeChallenge  string `json:"clientCodeChallenge"`
	ProviderCodeVerifier string `json:"providerCodeVerifier"`
	Intent               string `json:"intent"`
	// UserID 仅在绑定身份（intent=bind）时设置，记录发起授权的已登录用户。
	UserID    uint      `json:"userID,omitempty"`
	Next      string    `json:"next"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ProviderAuthGrant is the one-time handoff from the server callback to the
// public client. Sensitive provider codes and tokens never leave the server.
type ProviderAuthGrant struct {
	ProviderSlug string `json:"providerSlug"`
	ClientID     string `json:"clientID"`
	Intent       string `json:"intent"`
	UserID       uint   `json:"userID"`
	Subject      string `json:"subject"`
	// Profile 仅在绑定身份时携带：绑定在客户端兑换时才落库，回调阶段只保存已取回的资料。
	Profile      *ProviderAuthGrantProfile `json:"profile,omitempty"`
	ErrorCode    string                    `json:"errorCode,omitempty"`
	ErrorMessage string                    `json:"errorMessage,omitempty"`
	ErrorDetails string                    `json:"errorDetails,omitempty"`
	ExpiresAt    time.Time                 `json:"expiresAt"`
}

// ProviderAuthGrantProfile 是回调阶段从身份源取回、等待绑定的用户资料。
type ProviderAuthGrantProfile struct {
	DisplayName   string `json:"displayName"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
	ProfileJSON   string `json:"profileJSON"`
}

// ProviderAuthBridgeRepository stores and atomically consumes the short-lived
// transaction and grant records used by the provider auth bridge.
type ProviderAuthBridgeRepository interface {
	PutProviderAuthTransaction(ctx context.Context, id string, item ProviderAuthTransaction, ttl time.Duration) error
	ConsumeProviderAuthTransaction(ctx context.Context, id string) (*ProviderAuthTransaction, error)
	PutProviderAuthGrant(ctx context.Context, key string, item ProviderAuthGrant, ttl time.Duration) error
	ConsumeProviderAuthGrant(ctx context.Context, key string) (*ProviderAuthGrant, error)
}
