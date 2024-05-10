package outbound

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/nuts/iam"
	"github.com/SanteonNL/orca/orchestrator/nuts/vdr"
	"github.com/SanteonNL/orca/orchestrator/rest"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const fhirJSONContentType = "application/fhir+json"

func NewExchangeManager(baseURL *url.URL, nutsNodeAddress, ownDID, dataHolderDID string) *ExchangeManager {
	return &ExchangeManager{
		BaseURL:         baseURL,
		NutsNodeAddress: nutsNodeAddress,
		OwnDID:          ownDID,
		DataHolderDID:   dataHolderDID,
		exchanges:       make(map[string]Exchange),
		stateMutex:      &sync.Mutex{},
	}
}

type ExchangeManager struct {
	// BaseURL is the base location of where this application is hosted.
	BaseURL         *url.URL
	NutsNodeAddress string
	// OwnDID is the did:web DID of the local party.
	OwnDID string
	// DataHolderDID is the did:web DID of the data holder, at which the data is retrieved.
	// TODO: it is currently hardcoded, but in the future it should be derived from the shared CarePlan.
	DataHolderDID string
	// exchanges is a map of in-progress exchanges.
	// TODO: This should be persisted in session store (and/or cleaned up at some point).
	exchanges  map[string]Exchange
	stateMutex *sync.Mutex
}

// Exchange is an in-progress exchange.
type Exchange struct {
	// DataSource is the data source from which the data is retrieved.
	// In the future, it could retrieve data from multipe source, but currently there will be only 1
	DataSource        DataSource
	Result            *fhir.Bundle
	FHIROperationPath string
}

type DataSource struct {
	// NutsSessionID is the session ID of the Nuts user access token session.
	// It is used to retrieve the access token after the user completed authentication at the remote party.
	NutsSessionID string
	// FHIRBaseURL is the base URL of the FHIR API where the data is retrieved from.
	FHIRBaseURL string
	// DataHolderDID is the did:web DID of the data holder, at which the data is retrieved.
	DataHolderDID string
}

func (e *ExchangeManager) StartExchange(oauth2Scope string, fhirOperationPath string) (string, string, error) {
	if oauth2Scope == "" {
		return "", "", errors.New("oauth2 scope is required")
	}
	if fhirOperationPath == "" {
		return "", "", errors.New("FHIR operation path is required")
	}
	// TODO: Maybe Access Tokens could be re-used in future
	exchangeID := randomID()
	httpResponse, err := e.iamClient().RequestUserAccessToken(context.Background(), e.OwnDID, iam.RequestUserAccessTokenJSONRequestBody{
		// TODO: This should be provided by the caller or through the caller's Identity Provider.
		PreauthorizedUser: &iam.UserDetails{
			Id:   "1234567",
			Name: "L. Visser",
			Role: "Verpleegkundige niveau 4",
		},
		Scope:       oauth2Scope,
		RedirectUri: e.BaseURL.JoinPath("/exchange/" + exchangeID + "/callback").String(),
		Verifier:    e.DataHolderDID,
	})
	if err != nil {
		return "", "", fmt.Errorf("RequestUserAccessToken: %w", err)
	}
	response, err := iam.ParseRequestUserAccessTokenResponse(httpResponse)
	if err != nil {
		return "", "", fmt.Errorf("parse RequestUserAccessToken response: %w", err)
	}
	if response.ApplicationproblemJSONDefault != nil {
		return "", "", rest.RemoteAPIError{
			Err:    errors.New("RequestUserAccessToken failed"),
			Result: response.ApplicationproblemJSONDefault,
		}
	}
	if response.JSON200 == nil {
		return "", "", errors.New("RequestUserAccessToken failed: invalid response")
	}
	fhirBaseURL, err := e.findFHIRBaseURL(e.DataHolderDID)
	if err != nil {
		return "", "", fmt.Errorf("find FHIR base URL for '%s': %w", e.DataHolderDID, err)
	}
	e.storeExchange(exchangeID, Exchange{
		DataSource: DataSource{
			FHIRBaseURL:   fhirBaseURL,
			NutsSessionID: response.JSON200.SessionId,
			DataHolderDID: e.DataHolderDID,
		},
		FHIROperationPath: fhirOperationPath,
	})
	return exchangeID, response.JSON200.RedirectUri, nil
}

func (e *ExchangeManager) findFHIRBaseURL(holderDID string) (string, error) {
	request := &vdr.FilterServicesParams{
		Type:         new(string),
		EndpointType: new(vdr.FilterServicesParamsEndpointType),
	}
	// TODO: This should come from the use case
	*request.Type = "fhir-api"
	*request.EndpointType = vdr.String
	httpResponse, err := e.vdrClient().FilterServices(context.Background(), holderDID, request)
	if err != nil {
		return "", fmt.Errorf("FilterServices(): %w", err)
	}
	response, err := vdr.ParseFilterServicesResponse(httpResponse)
	if err != nil {
		return "", fmt.Errorf("parse FilterServices() response: %w", err)
	}
	if response.ApplicationproblemJSONDefault != nil {
		return "", rest.RemoteAPIError{
			Err:    errors.New("FilterServices failed"),
			Result: response.ApplicationproblemJSONDefault,
		}
	}
	if response.JSON200 == nil {
		return "", errors.New("FilterServices failed: invalid response")
	}
	if len(*response.JSON200) != 1 {
		return "", errors.New("FilterServices failed: expected exactly 1 DID document service")
	}
	return (*response.JSON200)[0].ServiceEndpoint.(string), nil
}

// HandleExchangeCallback handles the callback from the remote party after the user has completed the exchange.
// It is the trigger to retrieve the OAuth2 access token and do something with it (read data from external party).
func (e *ExchangeManager) HandleExchangeCallback(exchangeID string) error {
	exchange, err := e.loadExchange(exchangeID)
	if err != nil {
		return err
	}
	httpResponse, err := e.iamClient().RetrieveAccessToken(context.Background(), exchange.DataSource.NutsSessionID)
	if err != nil {
		return fmt.Errorf("retrieve access token: %w", err)
	}
	response, err := iam.ParseRetrieveAccessTokenResponse(httpResponse)
	if err != nil {
		return fmt.Errorf("parse retrieve access token response: %w", err)
	}
	if response.ApplicationproblemJSONDefault != nil {
		return rest.RemoteAPIError{
			Err:    errors.New("exchange failed"),
			Result: response.ApplicationproblemJSONDefault,
		}
	}
	if response.JSON200 == nil {
		return errors.New("RequestAccessToken failed: invalid response")
	}
	if err := e.retrieveData(exchange, *response.JSON200); err != nil {
		return err
	}
	// Exchanged finished
	e.storeExchange(exchangeID, *exchange)
	return nil
}

func (e *ExchangeManager) retrieveData(exchange *Exchange, tokenResponse iam.TokenResponse) error {
	baseURL, err := url.Parse(exchange.DataSource.FHIRBaseURL)
	if err != nil {
		return err
	}
	// TODO: system/value lookup yields 403 Forbidden for some reason
	//resourceURL := baseURL.JoinPath("/Patient")
	//queryParams := url.Values{}
	//queryParams.Set("identifier", fmt.Sprintf("%s|%s", *exchange.Patient.System, *exchange.Patient.Value))
	//resourceURL.RawQuery = queryParams.Encode()

	resourceURL, err := baseURL.Parse(exchange.FHIROperationPath)
	if err != nil {
		return fmt.Errorf("parse FHIR operation path: %w", err)
	}
	resourceURLStr := resourceURL.String()
	//:= baseURL.JoinPath("Patient/erXuFYUfucBZaryVksYEcMg3").String()
	data, err := readFHIRResource(resourceURLStr, tokenResponse)
	if err != nil {
		log.Info().Err(err).Msgf("Failed to read FHIR resource: %s", resourceURLStr)
		msg := err.Error()
		data, _ = json.Marshal(fhir.OperationOutcome{
			Issue: []fhir.OperationOutcomeIssue{
				{
					Severity:    fhir.IssueSeverityError,
					Code:        fhir.IssueTypeException,
					Diagnostics: &msg,
					Location:    []string{resourceURLStr},
				},
			},
		})
	}
	if exchange.Result == nil {
		exchange.Result = &fhir.Bundle{
			Type: fhir.BundleTypeSearchset,
		}
	}
	exchange.Result.Entry = append(exchange.Result.Entry, fhir.BundleEntry{
		FullUrl:  &resourceURLStr,
		Resource: data,
	})
	return nil
}

func readFHIRResource(resourceURL string, token iam.TokenResponse) ([]byte, error) {
	httpRequest, _ := http.NewRequest("GET", resourceURL, nil)
	httpRequest.Header.Add("Authorization", token.TokenType+" "+token.AccessToken)
	httpRequest.Header.Add("Accept", fhirJSONContentType)
	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("retrieve data failed: %w", err)
	}
	contentType := httpResponse.Header.Get("Content-Type")
	if !strings.Contains(contentType, fhirJSONContentType) {
		return nil, fmt.Errorf("retrieve data failed: unexpected content type %s (expected %s)", contentType, fhirJSONContentType)
	}
	data, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1024*1024*1024)) // 10mb seems about a right limit?
	if err != nil {
		return nil, fmt.Errorf("retrieve data failed: %w", err)
	}
	return data, nil
}

func (e *ExchangeManager) iamClient() *iam.Client {
	return &iam.Client{
		Server: e.NutsNodeAddress,
		Client: http.DefaultClient,
	}
}

func (e *ExchangeManager) vdrClient() *vdr.Client {
	return &vdr.Client{
		Server: e.NutsNodeAddress,
		Client: http.DefaultClient,
	}
}

func (e *ExchangeManager) deleteExchange(exchangeID string) {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()
	delete(e.exchanges, exchangeID)
}

func (e *ExchangeManager) storeExchange(exchangeID string, exchange Exchange) {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()
	e.exchanges[exchangeID] = exchange
}

func (e *ExchangeManager) loadExchange(exchangeID string) (*Exchange, error) {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()
	exchange, exists := e.exchanges[exchangeID]
	if !exists {
		return nil, errors.New("exchange not found")
	}
	return &exchange, nil
}

func randomID() string {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
