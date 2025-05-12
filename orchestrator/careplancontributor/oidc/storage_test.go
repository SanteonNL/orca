package oidc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"sync"
	"testing"
	"time"
)

func TestStorage_prune(t *testing.T) {
	t.Run("nothing to prune", func(t *testing.T) {
		storage := Storage{
			authRequests: map[string]AuthRequest{
				"1": {
					ID:             "1",
					ExpirationTime: time.Now().Add(1 * time.Hour),
				},
			},
			tokens: map[string]Token{
				"1": {
					ID:             "1",
					ExpirationTime: time.Now().Add(1 * time.Hour),
				},
			},
			mux: &sync.RWMutex{},
		}
		storage.prune(time.Now())
		assert.Len(t, storage.authRequests, 1)
		assert.Len(t, storage.tokens, 1)
	})
	t.Run("pruned", func(t *testing.T) {
		storage := Storage{
			authRequests: map[string]AuthRequest{
				"1": {
					ID:             "1",
					ExpirationTime: time.Now().Add(-1 * time.Hour),
				},
			},
			tokens: map[string]Token{
				"1": {
					ID:             "1",
					ExpirationTime: time.Now().Add(-1 * time.Hour),
				},
			},
			mux: &sync.RWMutex{},
		}
		storage.prune(time.Now())
		assert.Len(t, storage.authRequests, 0)
		assert.Len(t, storage.tokens, 0)
	})
}

func TestStorage_CreateAccessToken(t *testing.T) {
	storage := Storage{
		mux:    &sync.RWMutex{},
		tokens: map[string]Token{},
	}
	scopes := []string{"openid", "profile", "email"}
	tokenID, expiration, err := storage.CreateAccessToken(context.Background(), &AuthRequest{
		AuthRequest: oidc.AuthRequest{
			ClientID: "client",
			Scopes:   scopes,
		},
		User: &UserDetails{
			ID: "user",
		},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, tokenID)
	assert.NotEmpty(t, expiration)
	assert.Len(t, storage.tokens, 1)
	assert.Equal(t, tokenID, storage.tokens[tokenID].ID)
	assert.Equal(t, expiration, storage.tokens[tokenID].ExpirationTime)
	assert.Equal(t, scopes, storage.tokens[tokenID].Scopes)
}
