package nuts

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/knadh/koanf/maps"
	ssi "github.com/nuts-foundation/go-did"
	"github.com/nuts-foundation/go-nuts-client/nuts/discovery"
	"github.com/nuts-foundation/go-nuts-client/nuts/vcr"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sync"
	"time"
)

const identitiesCacheTTL = 5 * time.Minute

var uziOtherNameUraRegex = regexp.MustCompile("^[0-9.]+-\\d+-\\d+-S-(\\d+)-00\\.000-\\d+$")

// DutchNutsProfile is the Profile for running the SCP-node using the Nuts, with Dutch Verifiable Credential configuration and code systems.
// - Authentication: Nuts RFC021 Access Tokens
// - Care Services Discovery: Nuts Discovery Service
type DutchNutsProfile struct {
	Config                Config
	cachedIdentities      []fhir.Identifier
	identitiesRefreshedAt time.Time
	vcrClient             vcr.ClientWithResponsesInterface
	csd                   csd.Directory
}

func New(config Config) (*DutchNutsProfile, error) {
	vcrClient, err := vcr.NewClientWithResponses(config.API.URL)
	if err != nil {
		return nil, err
	}
	apiClient, _ := discovery.NewClientWithResponses(config.API.URL)
	return &DutchNutsProfile{
		Config:    config,
		vcrClient: vcrClient,
		csd: &CsdDirectory{
			APIClient:  apiClient,
			ServiceID:  config.DiscoveryService,
			entryCache: make(map[string]cacheEntry),
			cacheMux:   sync.RWMutex{},
		},
	}, nil
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
			log.Warn().Ctx(ctx).Err(err).Msg("Failed to refresh local identities using Nuts node")
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
	response, err := d.vcrClient.GetCredentialsInWalletWithResponse(ctx, d.Config.OwnSubject)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}
	if response.JSON200 == nil {
		if response.ApplicationproblemJSONDefault != nil {
			detail := fmt.Sprintf("HTTP %d - %s - %s", int(response.ApplicationproblemJSONDefault.Status), response.ApplicationproblemJSONDefault.Title, response.ApplicationproblemJSONDefault.Detail)
			return nil, fmt.Errorf("list credentials non-OK HTTP response (status=%s): %s", response.Status(), detail)
		}
		return nil, fmt.Errorf("list credentials non-OK HTTP response (status=%s)", response.Status())
	}
	var results []fhir.Identifier
	for _, cred := range *response.JSON200 {
		identifiers, err := d.identifiersFromCredential(cred)
		if err != nil {
			log.Warn().Ctx(ctx).Err(err).Msgf("Failed to extract identifiers from credential: %s", cred.ID)
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
		var ura string
		if cred.IsType(ssi.MustParseURI("NutsUraCredential")) {
			ura, _ = flattenCredential["credentialSubject.organization.ura"].(string)
		}
		if cred.IsType(ssi.MustParseURI("UziServerCertificateCredential")) {
			otherName, ok := flattenCredential["credentialSubject.otherName"].(string)
			if ok {
				if match := uziOtherNameUraRegex.FindStringSubmatch(otherName); len(match) > 1 {
					ura = match[1]
				}
			}
		}
		if ura != "" {
			results = append(results, fhir.Identifier{
				System: to.Ptr(coolfhir.URANamingSystem),
				Value:  to.Ptr(ura),
			})
		}
	}
	return results, nil
}
