package api

import (
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/outbound"
	"github.com/SanteonNL/orca/orchestrator/rest"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

var _ json.Marshaler = APIError{}

type APIError struct {
	Err error `json:"-"`
}

func (e APIError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"error": e.Err.Error(),
	})
}

type Service struct {
	ExchangeManager *outbound.ExchangeManager
}

type StartExchangeRequest struct {
	Patient fhir.Identifier `json:"patient"`
}

type StartExchangeResponse struct {
	ExchangeID  string `json:"exchange_id"`
	RedirectURI string `json:"redirect_uri"`
}

func (h Service) startExchange(response http.ResponseWriter, httpRequest *http.Request) {
	var request StartExchangeRequest
	if err := rest.UnmarshalJSONRequestBody(httpRequest, &request); err != nil {
		rest.RespondWithAPIError(response, err)
		return
	}
	exchangeID, redirectURI, err := h.ExchangeManager.StartExchange(request.Patient)
	if err != nil {
		rest.RespondWithAPIError(response, err)
		return
	}
	rest.RespondJSON(response, http.StatusOK, StartExchangeResponse{ExchangeID: exchangeID, RedirectURI: redirectURI})
}

func (h Service) Start(listenAddress string) error {
	httpHandler := http.NewServeMux()
	httpHandler.HandleFunc("POST /exchange", h.startExchange)
	err := http.ListenAndServe(listenAddress, httpHandler)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
