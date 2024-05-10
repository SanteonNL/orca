package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/outbound"
	"github.com/SanteonNL/orca/orchestrator/rest"
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

var _ StrictServerInterface = (*Service)(nil)

type Service struct {
	ExchangeManager *outbound.ExchangeManager
}

func (h Service) StartExchange(ctx context.Context, request StartExchangeRequestObject) (StartExchangeResponseObject, error) {
	exchange, redirectURI, err := h.ExchangeManager.StartExchange(request.Body.Oauth2Scope, request.Body.FhirOperationPath)
	if err != nil {
		return nil, err
	}
	return StartExchange201JSONResponse{
		ExchangeId:  exchange,
		RedirectUrl: redirectURI,
	}, nil
}

func (h Service) GetExchangeResult(ctx context.Context, request GetExchangeResultRequestObject) (GetExchangeResultResponseObject, error) {
	result, err := h.ExchangeManager.GetExchangeResult(request.Id)
	if errors.Is(err, outbound.ErrExchangeNotFound) {
		return GetExchangeResult404Response{}, nil
	}
	if errors.Is(err, outbound.ErrExchangeNotReady) {
		return GetExchangeResult409Response{}, nil
	}
	if err != nil {
		return nil, err
	}
	return GetExchangeResult200JSONResponse(result), nil
}

func (h Service) startExchange(response http.ResponseWriter, httpRequest *http.Request) {
	var request StartExchangeRequest
	if err := rest.UnmarshalJSONRequestBody(httpRequest, &request); err != nil {
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
