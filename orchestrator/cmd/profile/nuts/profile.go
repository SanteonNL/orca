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
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const identitiesCacheTTL = 5 * time.Minute
const nutsAuthorizationServerExtensionURL = "http://santeonnl.github.io/shared-care-planning/StructureDefinition/Nuts#AuthorizationServer"

var uziOtherNameUraRegex = regexp.MustCompile("^[0-9.]+-\\d+-\\d+-S-(\\d+)-00\\.000-\\d+$")
var oauthRestfulSecurityServiceCoding = fhir.Coding{
	System: to.Ptr("http://terminology.hl7.org/CodeSystem/restful-security-service"),
	Code:   to.Ptr("OAuth"),
}

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

func (d DutchNutsProfile) HttpClient(ctx context.Context, serverIdentity fhir.Identifier) (*http.Client, error) {
	if serverIdentity.System == nil || serverIdentity.Value == nil {
		return nil, fmt.Errorf("server identity must have system and value")
	}
	var authzServerURL string
	switch to.EmptyString(serverIdentity.System) {
	case "https://build.fhir.org/http.html#root":
		// FHIR base URL: need to look up CapabilityStatement
		capabilityStatement, err := d.readCapabilityStatement(ctx, *serverIdentity.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to read CapabilityStatement: %w", err)
		}
	outer:
		for _, rest := range capabilityStatement.Rest {
			if rest.Security != nil {
				for _, securityService := range rest.Security.Service {
					if coolfhir.ContainsCoding(oauthRestfulSecurityServiceCoding, securityService.Coding...) {
						for _, extension := range securityService.Extension {
							if extension.Url == nutsAuthorizationServerExtensionURL && extension.ValueString != nil {
								authzServerURL = *extension.ValueString
								break outer
							}
						}
					}
				}
			}
		}
	case coolfhir.URANamingSystem:
		// Care Plan Contributor: need to look up authz server URL in CSD
		authServerURLEndpoints, err := d.csd.LookupEndpoint(ctx, &serverIdentity, authzServerURLEndpointName)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup authz server URL (owner=%s): %w", coolfhir.ToString(serverIdentity), err)
		}
		if len(authServerURLEndpoints) == 0 {
			return nil, fmt.Errorf("no authz server URL found for owner %s", coolfhir.ToString(serverIdentity))
		}
		authzServerURL = authServerURLEndpoints[0].Address
	default:
		return nil, fmt.Errorf("unsupported server identity system: %s", *serverIdentity.System)
	}

	parsedAuthzServerURL, err := url.Parse(authzServerURL)
	if err != nil {
		return nil, err
	}
	client := oauth2.NewClient(nuts.OAuth2TokenSource{
		NutsSubject: d.Config.OwnSubject,
		NutsAPIURL:  d.Config.API.URL,
	}, careplancontributor.CarePlanServiceOAuth2Scope, parsedAuthzServerURL)
	if d.clientCert != nil {
		tlsConfig := globals.DefaultTLSConfig.Clone()
		tlsConfig.Certificates = []tls.Certificate{*d.clientCert}
		client.Transport = &http.Transport{TLSClientConfig: tlsConfig}
	} else {
		httpTransport := http.DefaultTransport.(*http.Transport).Clone()
		httpTransport.TLSClientConfig = globals.DefaultTLSConfig
		client.Transport = httpTransport
	}
	return client, nil
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

func (d DutchNutsProfile) readCapabilityStatement(ctx context.Context, fhirBaseURL string) (*fhir.CapabilityStatement, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, "GET", fhirBaseURL+"/metadata", nil)
	if err != nil {
		return nil, err
	}
	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 299 {
		return nil, fmt.Errorf("unexpected status code: %d", httpResponse.StatusCode)
	}
	var cp fhir.CapabilityStatement
	if err := json.NewDecoder(httpResponse.Body).Decode(&cp); err != nil {
		return nil, err
	}
	return &cp, nil
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

func (d DutchNutsProfile) CapabilityStatement(cp *fhir.CapabilityStatement) {
	for _, rest := range cp.Rest {
		if rest.Security == nil {
			rest.Security = &fhir.CapabilityStatementRestSecurity{}
		}
		rest.Security.Service = append(rest.Security.Service, fhir.CodeableConcept{
			Coding: []fhir.Coding{oauthRestfulSecurityServiceCoding},
			Extension: []fhir.Extension{
				{
					Url:         nutsAuthorizationServerExtensionURL,
					ValueString: to.Ptr(d.Config.Public.URL),
				},
			},
		})
	}
}
