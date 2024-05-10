package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/outbound"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

type FHIRBundle = fhir.Bundle

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

func (h Service) StartExchange(_ context.Context, request StartExchangeRequestObject) (StartExchangeResponseObject, error) {
	// TODO: initiator and remote party identifiers are still hardcoded here
	exchange, redirectURI, err := h.ExchangeManager.StartExchange(request.Body.Oauth2Scope, request.Body.FhirOperationPath, "clinic", "hospital")
	if err != nil {
		log.Warn().Err(err).Msg("Could not start exchange.")
		return nil, err
	}
	return StartExchange201JSONResponse{
		ExchangeId:  exchange,
		RedirectUrl: redirectURI,
	}, nil
}

func (h Service) GetExchangeResult(_ context.Context, request GetExchangeResultRequestObject) (GetExchangeResultResponseObject, error) {
	result := h.ExchangeManager.Get(request.Id)
	if result == nil {
		return GetExchangeResult404Response{}, nil
	}
	if result.Result == nil {
		return GetExchangeResult409Response{}, nil
	}
	return GetExchangeResult200JSONResponse(*result.Result), nil
}

func (h Service) Start(listenAddress string) error {
	httpService := echo.New()
	httpService.HideBanner = true
	httpService.HidePort = true
	RegisterHandlers(httpService, NewStrictHandler(h, nil))
	err := httpService.Start(listenAddress)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
