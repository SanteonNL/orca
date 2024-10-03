package nuts

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/knadh/koanf/maps"
	"github.com/nuts-foundation/go-nuts-client/nuts/vcr"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"path"
	"time"
)

const identitiesCacheTTL = 5 * time.Minute

// defaultCredentialMapping maps FHIR identifiers to attributes in Verifiable Credentials according to the Dutch Nuts Profile.
var defaultCredentialMapping = map[string]string{
	coolfhir.URANamingSystem: "credentialSubject.organization.ura", // NutsURACredential provides URA attribute
}

// DutchNutsProfile is the Profile for running the SCP-node using the Nuts, with Dutch Verifiable Credential configuration and code systems.
// - Authentication: Nuts RFC021 Access Tokens
// - Care Services Discovery: Nuts Discovery Service
type DutchNutsProfile struct {
	Config                Config
	cachedIdentities      []fhir.Identifier
	identitiesRefreshedAt time.Time
}

// RegisterHTTPHandlers registers the well-known OAuth2 Protected Resource HTTP endpoint that is used by OAuth2 Relying Parties to discover the OAuth2 Authorization Server.
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

// Identities consults the Nuts node to retrieve the local identities of the SCP node, given the credentials in the subject's wallet.
func (d *DutchNutsProfile) Identities(ctx context.Context) ([]fhir.Identifier, error) {
	if time.Since(d.identitiesRefreshedAt) > identitiesCacheTTL || len(d.cachedIdentities) == 0 {
		identifiers, err := d.identities(ctx)
		if err != nil {
			log.Logger.Warn().Err(err).Msg("Failed to refresh local identities using Nuts node")
			if d.cachedIdentities == nil {
				// If we don't have a cached value, we can't return anything, so return the error.
				return nil, fmt.Errorf("failed to load local identities: %w", err)
			}
		} else {
			d.cachedIdentities = identifiers
			d.identitiesRefreshedAt = time.Now()
		}
	}
	return d.cachedIdentities, nil
}

func (d DutchNutsProfile) identities(ctx context.Context) ([]fhir.Identifier, error) {
	vcrClient, err := vcr.NewClientWithResponses(d.Config.API.URL)
	if err != nil {
		return nil, err
	}
	response, err := vcrClient.GetCredentialsInWalletWithResponse(ctx, d.Config.OwnSubject)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}
	if response.JSON200 == nil {
		if response.ApplicationproblemJSONDefault != nil {
			if response.ApplicationproblemJSONDefault.Status == http.StatusNotFound {
				return nil, fmt.Errorf("%s - %s", response.ApplicationproblemJSONDefault.Title, response.ApplicationproblemJSONDefault.Detail)
			}
			return nil, fmt.Errorf("list credentials non-OK HTTP response (status=%s): %v", response.Status(), response.ApplicationproblemJSONDefault)
		}
		return nil, fmt.Errorf("list credentials non-OK HTTP response (status=%s)", response.Status())
	}
	var results []fhir.Identifier
	for _, cred := range *response.JSON200 {
		identifiers, err := d.identifiersFromCredential(cred)
		if err != nil {
			log.Logger.Warn().Err(err).Msgf("Failed to extract identifiers from credential: %s", cred.ID)
			continue
		}
		results = append(results, identifiers...)
	}
	return results, nil
}

func (d DutchNutsProfile) identifiersFromCredential(cred vcr.VerifiableCredential) ([]fhir.Identifier, error) {
	var asMaps []map[string]interface{}
	if err := cred.UnmarshalCredentialSubject(&asMaps); err != nil {
		return nil, err
	}
	var results []fhir.Identifier
	for _, asMap := range asMaps {
		flattenCredential, _ := maps.Flatten(asMap, []string{"credentialSubject"}, ".")
		for namingSystem, jsonPath := range defaultCredentialMapping {
			identifierValue, ok := flattenCredential[jsonPath]
			if !ok {
				continue
			}
			results = append(results, fhir.Identifier{
				System: &namingSystem,
				Value:  to.Ptr(fmt.Sprintf("%s", identifierValue)),
			})
		}
	}
	return results, nil
}
