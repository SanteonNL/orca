package user

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const sessionLifetime = 15 * time.Minute

// NewSessionManager creates a new session manager.
// It uses in-memory storage.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		store: &sessionStore{
			sessions: make(map[string]*Session),
			mux:      &sync.Mutex{},
		},
	}
}

type Session struct {
	Data    SessionData
	Expires time.Time
}

type SessionData struct {
	FHIRLauncher string
	StringValues map[string]string
	OtherValues  map[string]interface{}
}

type SessionManager struct {
	store *sessionStore
}

type sessionStore struct {
	sessions map[string]*Session
	mux      *sync.Mutex
}

// Create creates a new session and sets a session cookie.
// The given values are stored in the session, which can be retrieved later using Get.
func (m *SessionManager) Create(response http.ResponseWriter, values SessionData) {
	id, _ := m.store.create(values)
	setSessionCookie(id, response)
}

// Get retrieves the session for the given request.
// The session is retrieved using the session cookie.
// If no session is found, nil is returned.
func (m *SessionManager) Get(request *http.Request) *SessionData {
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

func (m *SessionManager) Destroy(response http.ResponseWriter, request *http.Request) {
	sessionID := getSessionCookie(request)
	if sessionID != "" {
		m.store.destroy(sessionID)
	} else {
		log.Warn().Msg("No session to destroy")
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
func (m *SessionManager) PruneSessions() {
	m.store.mux.Lock()
	defer m.store.mux.Unlock()
	m.store.prune()
}

func (s *sessionStore) create(values SessionData) (string, *Session) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.prune()
	// prevent nil derefs later on
	if values.OtherValues == nil {
		values.OtherValues = make(map[string]interface{})
	}
	if values.StringValues == nil {
		values.StringValues = make(map[string]string)
	}
	result := &Session{
		Data:    values,
		Expires: time.Now().Add(sessionLifetime),
	}
	id := uuid.NewString()
	s.sessions[id] = result
	return id, result
}

func (s *sessionStore) get(id string) *Session {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.prune()
	return s.sessions[id]
}

func (s *sessionStore) prune() {
	for id, session := range s.sessions {
		if session.Expires.Before(time.Now()) {
			delete(s.sessions, id)
		}
	}
}

func (s *sessionStore) destroy(id string) {
	log.Info().Msgf("Destroying user session (id=%s)", id)
	s.mux.Lock()
	defer s.mux.Unlock()
	delete(s.sessions, id)
}

func setSessionCookie(sessionID string, response http.ResponseWriter) {
	// TODO: Maybe makes this a __Host or __Secure cookie?
	cookie := http.Cookie{
		Name:     "sid",
		Value:    sessionID,
		HttpOnly: true,
		Expires:  time.Now().Add(sessionLifetime),
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

func (m *SessionManager) SessionCount() int {
	return len(m.store.sessions)
}
