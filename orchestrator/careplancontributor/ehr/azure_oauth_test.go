package ehr

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAzureCredential(t *testing.T) {
	expected := map[string]interface{}{
		"access_token":  "X",
		"refresh_token": "Y",
		"expires_in":    3600,
	}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		marshal, err := json.Marshal(expected)
		if err != nil {
			t.Errorf("failed to marshal response: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(marshal)
		if err != nil {
			t.Errorf("failed to marshal response: %v", err)
		}
	}))
	defer svr.Close()

	tests := []struct {
		name    string
		prepare func() func()
		wantErr bool
	}{
		{
			name: "success",
			prepare: func() func() {
				t.Setenv("IDENTITY_ENDPOINT", svr.URL)
				t.Setenv("IDENTITY_HEADER", "X")
				return func() {
					t.Setenv("IDENTITY_ENDPOINT", "")
					t.Setenv("IDENTITY_HEADER", "")
				}

			},
			wantErr: false,
		},
		{
			name: "failure",
			prepare: func() func() {
				return func() {}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.prepare()
			defer cleanup()

			client, err := newAzureOauthClient()
			if err != nil {
				t.Errorf("newAzureOauthClient() error = %v", err)
				return
			}
			cred, err := client.GetAzureCredential()
			_, err = client.GetBearerToken(
				context.Background(),
				cred,
				"example.com:443")

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAzureCredential() error = %v, err %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		err      error
		token    string
	}{
		{
			name:     "valid token",
			endpoint: "example.com:443",
			err:      nil,
			token:    "fake_token",
		},
		{
			name:     "invalid scope",
			endpoint: "invalid.endpoint",
			err:      errors.New("Err"),
		},
		{
			name:     "failed to retrieve token",
			endpoint: "invalid.scope",
			err:      errors.New("Err"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			client := &AzureOauthClientImpl{}
			cred := fake.TokenCredential{}
			if tt.err != nil {
				cred.SetError(tt.err)
			}
			tok, err := client.GetBearerToken(context.Background(), &cred, tt.endpoint)
			if !errors.Is(err, tt.err) {
				t.Errorf("GetBearerToken() error = %v, err %v", err, tt.err)
			}
			if err == nil && tok.Token != tt.token {
				t.Errorf("GetBearerToken() token = %v, wantToken %v", tok.Token, tt.token)
			}
		})
	}
}

func TestGetScopeFromEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "valid endpoint",
			endpoint: "example.com:443",
			want:     "https://example.com",
		},
		{
			name:     "missing port",
			endpoint: "example.com",
			want:     "https://example.com",
		},
		{
			name:     "empty endpoint",
			endpoint: "",
			want:     "https://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getScopeFromEndpoint(tt.endpoint); got != tt.want {
				t.Errorf("getScopeFromEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}
