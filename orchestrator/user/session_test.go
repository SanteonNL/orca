package user

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test_SessionManager_SessionLifecycle tests the lifecycle of a session. It creates a session, retrieves it, deletes it and verifies that it is deleted.
func Test_SessionManager_SessionLifecycle(t *testing.T) {
	sessionManager := NewSessionManager(time.Minute)
	response := httptest.NewRecorder()

	// Create a session
	sessionManager.Create(response, SessionData{
		StringValues: map[string]string{
			"key": "value",
		},
	})
	require.Equal(t, 1, len(sessionManager.store.sessions))
	require.Equal(t, http.StatusOK, response.Code)

	request := httptest.NewRequest("GET", "/", nil)

	// Get session without cookie returns nil data
	sessionData := sessionManager.Get(request)
	require.Nil(t, sessionData)

	request.Header.Set("Cookie", response.Header().Get("Set-Cookie"))

	// Get the session
	sessionData = sessionManager.Get(request)
	require.NotNil(t, sessionData)
	// Check values in the request
	require.Equal(t, "value", sessionData.StringValues["key"])

	// Create new session to validate that delete only deletes the session in request
	response = httptest.NewRecorder()
	sessionManager.Create(response, SessionData{
		StringValues: map[string]string{
			"key2": "value2",
		},
	})
	require.Equal(t, 2, len(sessionManager.store.sessions))
	require.Equal(t, http.StatusOK, response.Code)

	// store the cookie for later
	cookie := response.Header().Get("Set-Cookie")

	// Delete the first session, using the existing request
	response = httptest.NewRecorder()
	sessionManager.Destroy(response, request)

	parts := strings.Split(response.Header().Get("Set-Cookie"), ";")
	require.Equal(t, "sid=", parts[0])
	// Verify the expiration date is in the past
	expiration := strings.TrimPrefix(parts[1], " Expires=")
	require.NotEmpty(t, expiration)
	expirationTime, err := http.ParseTime(expiration)
	require.NoError(t, err)
	require.True(t, expirationTime.Before(time.Now()))

	require.Equal(t, 1, len(sessionManager.store.sessions))

	// Check that the second session is still there
	request.Header.Set("Cookie", cookie)
	sessionData = sessionManager.Get(request)
	require.NotNil(t, sessionData)
	require.Equal(t, "value2", sessionData.StringValues["key2"])
}
