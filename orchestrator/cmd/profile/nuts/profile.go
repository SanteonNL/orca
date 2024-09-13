package nuts

import (
	"encoding/json"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
	"net/http"
	"net/url"
	"path"
)

// DutchNutsProfile is the Profile for running the SCP-node using the Nuts, with Dutch Verifiable Credential configuration and code systems.
// - Authentication: Nuts RFC021 Access Tokens
// - Care Services Discovery: Nuts Discovery Service
type DutchNutsProfile struct {
	Config Config
}

// RegisterHTTPHandlers registers an
func (d DutchNutsProfile) RegisterHTTPHandlers(basePath string, resourceServerURL *url.URL, mux *http.ServeMux) {
	mux.HandleFunc("GET "+path.Join("/", basePath, "/.well-known/oauth-protected-resource"), func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		md := oauth2.ProtectedResourceMetadata{
			Resource:               resourceServerURL.String(),
			AuthorizationServers:   []string{d.Config.Public.Parse().JoinPath("oauth2", d.Config.OwnSubject).String()},
			BearerMethodsSupported: []string{"header"},
		}
		_ = json.NewEncoder(writer).Encode(md)
	})
}
