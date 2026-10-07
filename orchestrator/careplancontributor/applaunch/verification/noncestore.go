package verification

import (
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
)

// nonceStore holds prepared launch sessions until the browser redeems them. A nonce is single-use and
// short-lived, so the launch URL behaves like an authorization code: the authenticated POST proves who
// asked, the unauthenticated GET only converts that proof into a session cookie.
type nonceStore struct {
	cache *ttlcache.Cache[string, session.Data]
}

func newNonceStore(ttl time.Duration) *nonceStore {
	cache := ttlcache.New[string, session.Data](ttlcache.WithTTL[string, session.Data](ttl))
	go cache.Start()
	return &nonceStore{cache: cache}
}

func (s *nonceStore) Put(data session.Data) string {
	nonce := uuid.NewString()
	s.cache.Set(nonce, data, ttlcache.DefaultTTL)
	return nonce
}

// Take returns the session for a nonce exactly once; expired or unknown nonces return false.
func (s *nonceStore) Take(nonce string) (session.Data, bool) {
	item, ok := s.cache.GetAndDelete(nonce)
	if !ok {
		return session.Data{}, false
	}
	return item.Value(), true
}
