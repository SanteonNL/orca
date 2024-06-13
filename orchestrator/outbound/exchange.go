package outbound

//
//import (
//	"context"
//	"crypto/rand"
//	"encoding/base64"
//	"encoding/json"
//	"errors"
//	"fmt"
//	"github.com/SanteonNL/orca/orchestrator/nuts/iam"
//	"github.com/SanteonNL/orca/orchestrator/nuts/vdr"
//	"github.com/SanteonNL/orca/orchestrator/rest"
//	"github.com/rs/zerolog/log"
//	"github.com/samply/golang-fhir-models/fhir-models/fhir"
//	"io"
//	"net/http"
//	"net/url"
//	"strings"
//	"sync"
//)
//
//const fhirJSONContentType = "application/fhir+json"
//
//func NewExchangeManager(baseURL *url.URL, nutsNodeAddress string, didResolver DIDResolver) *ExchangeManager {
//	return &ExchangeManager{
//		BaseURL:         baseURL,
//		NutsNodeAddress: nutsNodeAddress,
//		exchanges:       make(map[string]Exchange),
//		didResolver:     didResolver,
//		stateMutex:      &sync.Mutex{},
//	}
//}
//
//type ExchangeManager struct {
//	// BaseURL is the base location of where this application is hosted.
//	BaseURL         *url.URL
//	NutsNodeAddress string
//	// exchanges is a map of in-progress exchanges.
//	// TODO: This should be persisted in session store (and/or cleaned up at some point).
//	exchanges   map[string]Exchange
//	stateMutex  *sync.Mutex
//	didResolver DIDResolver
//}
//
//// Exchange is an in-progress exchange.
//type Exchange struct {
//	// DataSource is the data source from which the data is retrieved.
//	// In the future, it could retrieve data from multipe source, but currently there will be only 1
//	DataSource        DataSource
//	Result            *fhir.Bundle
//	FHIROperationPath string
//}
//
//type DataSource struct {
//	// NutsSessionID is the session ID of the Nuts user access token session.
//	// It is used to retrieve the access token after the user completed authentication at the remote party.
//	NutsSessionID string
//	// FHIRBaseURL is the base URL of the FHIR API where the data is retrieved from.
//	FHIRBaseURL string
//	// DataHolderDID is the did:web DID of the data holder, at which the data is retrieved.
//	DataHolderDID string
//}
//
//func (e *ExchangeManager) StartExchange(oauth2Scope string, fhirOperationPath string, initiatorIdentifier string, dataHolderIdentifier string) (string, string, error) {
//	log.Info().Msgf("Starting exchange (initiator: %s, data holder: %s, scope: %s)", initiatorIdentifier, dataHolderIdentifier, oauth2Scope)
//	ownDID, err := e.didResolver.Resolve(initiatorIdentifier)
//	if err != nil {
//		return "", "", err
//	}
//	dataHolderDID, err := e.didResolver.Resolve(dataHolderIdentifier)
//	if err != nil {
//		return "", "", err
//	}
//
//	if oauth2Scope == "" {
//		return "", "", errors.New("oauth2 scope is required")
//	}
//	if fhirOperationPath == "" {
//		return "", "", errors.New("FHIR operation path is required")
//	}
//	// TODO: Maybe Access Tokens could be re-used in future
//	exchangeID := randomID()
//	httpResponse, err := e.iamClient().RequestUserAccessToken(context.Background(), ownDID, iam.RequestUserAccessTokenJSONRequestBody{
//		// TODO: This should be provided by the caller or through the caller's Identity Provider.
//		PreauthorizedUser: &iam.UserDetails{
//			Id:   "1234567",
//			Name: "L. Visser",
//			Role: "Verpleegkundige niveau 4",
//		},
//		Scope:       oauth2Scope,
//		RedirectUri: e.BaseURL.JoinPath("/exchange/" + exchangeID + "/callback").String(),
//		Verifier:    dataHolderDID,
//	})
//	if err != nil {
//		return "", "", fmt.Errorf("RequestUserAccessToken: %w", err)
//	}
//	response, err := iam.ParseRequestUserAccessTokenResponse(httpResponse)
//	if err != nil {
//		return "", "", fmt.Errorf("parse RequestUserAccessToken response: %w", err)
//	}
//	if response.ApplicationproblemJSONDefault != nil {
//		return "", "", rest.RemoteAPIError{
//			Err:    errors.New("RequestUserAccessToken failed"),
//			Result: response.ApplicationproblemJSONDefault,
//		}
//	}
//	if response.JSON200 == nil {
//		return "", "", errors.New("RequestUserAccessToken failed: invalid response")
//	}
//	fhirBaseURL, err := e.findFHIRBaseURL(dataHolderDID)
//	if err != nil {
//		return "", "", fmt.Errorf("find FHIR base URL for '%s': %w", dataHolderDID, err)
//	}
//	e.storeExchange(exchangeID, Exchange{
//		DataSource: DataSource{
//			FHIRBaseURL:   fhirBaseURL,
//			NutsSessionID: response.JSON200.SessionId,
//			DataHolderDID: dataHolderDID,
//		},
//		FHIROperationPath: fhirOperationPath,
//	})
//	return exchangeID, response.JSON200.RedirectUri, nil
//}
//
//func (e *ExchangeManager) findFHIRBaseURL(holderDID string) (string, error) {
//	request := &vdr.FilterServicesParams{
//		Type:         new(string),
//		EndpointType: new(vdr.FilterServicesParamsEndpointType),
//	}
//	// TODO: This should come from the use case
//	*request.Type = "fhir-api"
//	*request.EndpointType = vdr.String
//	httpResponse, err := e.vdrClient().FilterServices(context.Background(), holderDID, request)
//	if err != nil {
//		return "", fmt.Errorf("FilterServices(): %w", err)
//	}
//	response, err := vdr.ParseFilterServicesResponse(httpResponse)
//	if err != nil {
//		return "", fmt.Errorf("parse FilterServices() response: %w", err)
//	}
//	if response.ApplicationproblemJSONDefault != nil {
//		return "", rest.RemoteAPIError{
//			Err:    errors.New("FilterServices failed"),
//			Result: response.ApplicationproblemJSONDefault,
//		}
//	}
//	if response.JSON200 == nil {
//		return "", errors.New("FilterServices failed: invalid response")
//	}
//	if len(*response.JSON200) != 1 {
//		return "", errors.New("FilterServices failed: expected exactly 1 DID document service")
//	}
//	return (*response.JSON200)[0].ServiceEndpoint.(string), nil
//}
//
//// HandleExchangeCallback handles the callback from the remote party after the user has completed the exchange.
//// It is the trigger to retrieve the OAuth2 access token and do something with it (read data from external party).
//func (e *ExchangeManager) HandleExchangeCallback(exchangeID string) error {
//	log.Info().Msgf("Handling callback for exchange %s", exchangeID)
//	exchange := e.loadExchange(exchangeID)
//	if exchange == nil {
//		return errors.New("exchange not found")
//	}
//	httpResponse, err := e.iamClient().RetrieveAccessToken(context.Background(), exchange.DataSource.NutsSessionID)
//	if err != nil {
//		return fmt.Errorf("retrieve access token: %w", err)
//	}
//	response, err := iam.ParseRetrieveAccessTokenResponse(httpResponse)
//	if err != nil {
//		return fmt.Errorf("parse retrieve access token response: %w", err)
//	}
//	if response.ApplicationproblemJSONDefault != nil {
//		return rest.RemoteAPIError{
//			Err:    errors.New("exchange failed"),
//			Result: response.ApplicationproblemJSONDefault,
//		}
//	}
//	if response.JSON200 == nil {
//		return errors.New("RequestAccessToken failed: invalid response")
//	}
//	if err := e.retrieveData(exchange, *response.JSON200); err != nil {
//		return err
//	}
//	// Exchanged finished
//	log.Info().Msgf("Exchange %s completed, results available", exchangeID)
//	e.storeExchange(exchangeID, *exchange)
//	return nil
//}
//
//func (e *ExchangeManager) Get(exchangeID string) *Exchange {
//	return e.loadExchange(exchangeID)
//}
//
//func (e *ExchangeManager) retrieveData(exchange *Exchange, tokenResponse iam.TokenResponse) error {
//	resourceURL, err := buildFHIRResourceURL(exchange.DataSource.FHIRBaseURL, exchange.FHIROperationPath)
//	if err != nil {
//		return err
//	}
//	//:= baseURL.JoinPath("Patient/erXuFYUfucBZaryVksYEcMg3").String()
//	data, err := readFHIRResource(resourceURL, tokenResponse)
//	if err != nil {
//		log.Info().Err(err).Msgf("Failed to read FHIR resource: %s", resourceURL)
//		msg := err.Error()
//		data, _ = json.Marshal(fhir.OperationOutcome{
//			Issue: []fhir.OperationOutcomeIssue{
//				{
//					Severity:    fhir.IssueSeverityError,
//					Code:        fhir.IssueTypeException,
//					Diagnostics: &msg,
//					Location:    []string{resourceURL},
//				},
//			},
//		})
//	}
//	if exchange.Result == nil {
//		exchange.Result = &fhir.Bundle{
//			Type: fhir.BundleTypeSearchset,
//		}
//	}
//	exchange.Result.Entry = append(exchange.Result.Entry, fhir.BundleEntry{
//		FullUrl:  &resourceURL,
//		Resource: data,
//	})
//	return nil
//}
//
//func buildFHIRResourceURL(baseURL string, fhirOperationPath string) (string, error) {
//	parsedBaseURL, err := url.Parse(baseURL)
//	if err != nil {
//		return "", fmt.Errorf("invalid FHIR base URL (%s): %w", baseURL, err)
//	}
//	parsedFHIROperationPath, err := url.Parse(fhirOperationPath)
//	if err != nil {
//		return "", fmt.Errorf("invalid FHIR operation path: %w", err)
//	}
//	// TODO: system/value lookup yields 403 Forbidden for some reason
//	//resourceURL := baseURL.JoinPath("/Patient")
//	//queryParams := url.Values{}
//	//queryParams.Set("identifier", fmt.Sprintf("%s|%s", *exchange.Patient.System, *exchange.Patient.Value))
//	//resourceURL.RawQuery = queryParams.Encode()
//	resourceURL := parsedBaseURL.JoinPath(parsedFHIROperationPath.Path)
//	resourceURL.RawQuery = parsedFHIROperationPath.RawQuery
//	return resourceURL.String(), nil
//}
//
//func readFHIRResource(resourceURL string, token iam.TokenResponse) ([]byte, error) {
//	log.Debug().Msgf("Reading FHIR resource: %s", resourceURL)
//	httpRequest, _ := http.NewRequest("GET", resourceURL, nil)
//	httpRequest.Header.Add("Authorization", token.TokenType+" "+token.AccessToken)
//	httpRequest.Header.Add("Accept", fhirJSONContentType)
//	httpResponse, err := http.DefaultClient.Do(httpRequest)
//	if err != nil {
//		return nil, fmt.Errorf("retrieve data failed: %w", err)
//	}
//	contentType := httpResponse.Header.Get("Content-Type")
//	if !strings.Contains(contentType, fhirJSONContentType) {
//		return nil, fmt.Errorf("retrieve data failed: unexpected content type %s (expected %s)", contentType, fhirJSONContentType)
//	}
//	const maxResourceSize = 1024 * 1024 * 1024
//	data, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxResourceSize+1)) // 10mb seems about a right limit?
//	if err != nil {
//		return nil, fmt.Errorf("retrieve data failed: %w", err)
//	}
//	if len(data) > maxResourceSize {
//		return nil, fmt.Errorf("retrieved FHIR resource exceeds max. size of %d bytes", maxResourceSize)
//	}
//	return data, nil
//}
//
//func (e *ExchangeManager) iamClient() *iam.Client {
//	return &iam.Client{
//		Server: e.NutsNodeAddress,
//		Client: http.DefaultClient,
//	}
//}
//
//func (e *ExchangeManager) vdrClient() *vdr.Client {
//	return &vdr.Client{
//		Server: e.NutsNodeAddress,
//		Client: http.DefaultClient,
//	}
//}
//
//func (e *ExchangeManager) deleteExchange(exchangeID string) {
//	e.stateMutex.Lock()
//	defer e.stateMutex.Unlock()
//	delete(e.exchanges, exchangeID)
//}
//
//func (e *ExchangeManager) storeExchange(exchangeID string, exchange Exchange) {
//	e.stateMutex.Lock()
//	defer e.stateMutex.Unlock()
//	e.exchanges[exchangeID] = exchange
//}
//
//func (e *ExchangeManager) loadExchange(exchangeID string) *Exchange {
//	e.stateMutex.Lock()
//	defer e.stateMutex.Unlock()
//	ex := e.exchanges[exchangeID]
//	return &ex
//}
//
//func randomID() string {
//	buf := make([]byte, 32)
//	_, err := rand.Read(buf)
//	if err != nil {
//		log.Fatal().Err(err).Msg("Failed to generate random ID")
//	}
//	return base64.RawURLEncoding.EncodeToString(buf)
//}
