package user

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// NewSessionManager creates a new session manager.
// It uses in-memory storage.
func NewSessionManager[TData any](sessionLifetime time.Duration) *SessionManager[TData] {
	return &SessionManager[TData]{
		sessionLifetime: sessionLifetime,
		store: &sessionStore[TData]{
			sessions: make(map[string]*Session[TData]),
			mux:      &sync.Mutex{},
		},
	}
}

type Session[TData any] struct {
	Data    TData
	Expires time.Time
}

type SessionManager[TData any] struct {
	store           *sessionStore[TData]
	sessionLifetime time.Duration
}

type sessionStore[TData any] struct {
	sessions map[string]*Session[TData]
	mux      *sync.Mutex
}

// Create creates a new session and sets a session cookie.
// The given values are stored in the session, which can be retrieved later using Get.
func (m *SessionManager[TData]) Create(response http.ResponseWriter, values TData) {
	id, _ := m.store.create(values, m.sessionLifetime)
	setSessionCookie(id, response, m.sessionLifetime)
}

// Get retrieves the session for the given request.
// The session is retrieved using the session cookie.
// If no session is found, nil is returned.
func (m *SessionManager[TData]) Get(request *http.Request) *TData {
	sessionID := getSessionCookie(request)
	if sessionID == "" {
		return nil
	}
	session := m.store.get(sessionID)
	if session == nil {
		return nil
	}
	return &session.Data
}

func (m *SessionManager[TData]) Destroy(response http.ResponseWriter, request *http.Request) {
	sessionID := getSessionCookie(request)
	if sessionID != "" {
		m.store.destroy(sessionID)
	} else {
		log.Ctx(request.Context()).Warn().Msg("No session to destroy")
	}
	cookie := http.Cookie{
		Name:     "sid",
		Value:    "",
		HttpOnly: true,
		Expires:  time.Now().Add(-time.Minute),
	}
	http.SetCookie(response, &cookie)
}

// PruneSessions removes expired sessions.
func (m *SessionManager[TData]) PruneSessions() {
	m.store.mux.Lock()
	defer m.store.mux.Unlock()
	m.store.prune()
}

func (s *sessionStore[TData]) create(values TData, sessionLifetime time.Duration) (string, *Session[TData]) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.prune()
	result := &Session[TData]{
		Data:    values,
		Expires: time.Now().Add(sessionLifetime),
	}
	id := uuid.NewString()
	s.sessions[id] = result
	return id, result
}

func (s *sessionStore[TData]) get(id string) *Session[TData] {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.prune()
	return s.sessions[id]
}

func (s *sessionStore[TData]) prune() {
	for id, session := range s.sessions {
		if session.Expires.Before(time.Now()) {
			delete(s.sessions, id)
		}
	}
}

func (s *sessionStore[TData]) destroy(id string) {
	log.Info().Msgf("Destroying user session (id=%s)", id)
	s.mux.Lock()
	defer s.mux.Unlock()
	delete(s.sessions, id)
}

func setSessionCookie(sessionID string, response http.ResponseWriter, lifetime time.Duration) {
	// TODO: Maybe makes this a __Host or __Secure cookie?
	cookie := http.Cookie{
		Name:     "sid",
		Value:    sessionID,
		HttpOnly: true,
		Expires:  time.Now().Add(lifetime),
		Path:     "/",
	}
	http.SetCookie(response, &cookie)
}

func getSessionCookie(request *http.Request) string {
	cookie, err := request.Cookie("sid")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (m *SessionManager[TData]) SessionCount() int {
	return len(m.store.sessions)
}
