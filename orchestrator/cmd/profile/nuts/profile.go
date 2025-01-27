package nuts

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/knadh/koanf/maps"
	ssi "github.com/nuts-foundation/go-did"
	"github.com/nuts-foundation/go-nuts-client/nuts"
	"github.com/nuts-foundation/go-nuts-client/nuts/discovery"
	"github.com/nuts-foundation/go-nuts-client/nuts/vcr"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
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
	cachedIdentities      []fhir.Organization
	identitiesRefreshedAt time.Time
	vcrClient             vcr.ClientWithResponsesInterface
	csd                   csd.Directory
	clientCert            *tls.Certificate
}

func New(config Config) (*DutchNutsProfile, error) {
	var clientCert *tls.Certificate
	if config.AzureKeyVault.ClientCertName != "" {
		if config.AzureKeyVault.CredentialType == "" {
			config.AzureKeyVault.CredentialType = "managed_identity"
		}
		azKeysClient, err := azkeyvault.NewKeysClient(config.AzureKeyVault.URL, config.AzureKeyVault.CredentialType, false)
		if err != nil {
			return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
		}
		azCertClient, err := azkeyvault.NewCertificatesClient(config.AzureKeyVault.URL, config.AzureKeyVault.CredentialType, false)
		if err != nil {
			return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
		}
		clientCert, err = azkeyvault.GetTLSCertificate(context.Background(), azCertClient, azKeysClient, config.AzureKeyVault.ClientCertName)
		if err != nil {
			return nil, fmt.Errorf("unable to get client certificate from Azure Key Vault: %w", err)
		}
	} else {
		log.Warn().Msg("Nuts: no TLS client certificate configured for outbound HTTP requests")
	}

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
		clientCert: clientCert,
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

func (d DutchNutsProfile) HttpClient() *http.Client {
	var roundTripper http.RoundTripper
	if d.clientCert != nil {
		tlsConfig := globals.DefaultTLSConfig.Clone()
		tlsConfig.Certificates = []tls.Certificate{*d.clientCert}
		roundTripper = &http.Transport{TLSClientConfig: tlsConfig}
	} else {
		httpTransport := http.DefaultTransport.(*http.Transport).Clone()
		httpTransport.TLSClientConfig = globals.DefaultTLSConfig
		roundTripper = httpTransport
	}
	return &http.Client{
		Transport: &oauth2.Transport{
			UnderlyingTransport: roundTripper,
			TokenSource: nuts.OAuth2TokenSource{
				NutsSubject: d.Config.OwnSubject,
				NutsAPIURL:  d.Config.API.URL,
			},
			MetadataLoader: &oauth2.MetadataLoader{},
			AuthzServerLocators: []oauth2.AuthorizationServerLocator{
				oauth2.ProtectedResourceMetadataLocator,
			},
			Scope: careplancontributor.CarePlanServiceOAuth2Scope,
		},
	}
}

// Identities consults the Nuts node to retrieve the local identities of the SCP node, given the credentials in the subject's wallet.
func (d *DutchNutsProfile) Identities(ctx context.Context) ([]fhir.Organization, error) {
	if time.Since(d.identitiesRefreshedAt) > identitiesCacheTTL || len(d.cachedIdentities) == 0 {
		identifiers, err := d.identities(ctx)
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("Failed to refresh local identities using Nuts node")
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

func (d DutchNutsProfile) identities(ctx context.Context) ([]fhir.Organization, error) {
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
	var results []fhir.Organization
	for _, cred := range *response.JSON200 {
		identities, err := d.identifiersFromCredential(cred)
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Msgf("Failed to extract identities from credential: %s", cred.ID)
			continue
		}
		results = append(results, identities...)
	}
	// Deduplicate the organization entries, then build a sorted slice of the results
	deduplicated := make(map[string]fhir.Organization)
	for _, entry := range results {
		deduplicated[*entry.Identifier[0].Value] = entry
	}
	var deduplicatedResults []fhir.Organization
	for _, entry := range deduplicated {
		deduplicatedResults = append(deduplicatedResults, entry)
	}
	slices.SortStableFunc(deduplicatedResults, func(a, b fhir.Organization) int {
		return strings.Compare(*a.Identifier[0].Value, *b.Identifier[0].Value)
	})
	return deduplicatedResults, nil
}

func (d DutchNutsProfile) identifiersFromCredential(cred vcr.VerifiableCredential) ([]fhir.Organization, error) {
	var asMaps []map[string]interface{}
	if err := cred.UnmarshalCredentialSubject(&asMaps); err != nil {
		return nil, err
	}
	var results []fhir.Organization
	for _, asMap := range asMaps {
		flattenCredential, _ := maps.Flatten(asMap, []string{"credentialSubject"}, ".")
		var ura string
		var name string
		if cred.IsType(ssi.MustParseURI("NutsUraCredential")) {
			ura, _ = flattenCredential["credentialSubject.organization.ura"].(string)
			name, _ = flattenCredential["credentialSubject.organization.name"].(string)
		}
		if cred.IsType(ssi.MustParseURI("X509Credential")) {
			otherName, ok := flattenCredential["credentialSubject.san.otherName"].(string)
			if ok {
				if match := uziOtherNameUraRegex.FindStringSubmatch(otherName); len(match) > 1 {
					ura = match[1]
				}
			}
			name, _ = flattenCredential["credentialSubject.subject.O"].(string)
		}
		if ura != "" {
			entry := fhir.Organization{
				Identifier: []fhir.Identifier{
					{
						System: to.Ptr(coolfhir.URANamingSystem),
						Value:  to.Ptr(ura),
					},
				},
			}
			if name != "" {
				entry.Name = to.Ptr(name)
			}
			results = append(results, entry)
		}
	}
	return results, nil
}
