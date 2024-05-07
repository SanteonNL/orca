package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/outbound"
	"github.com/SanteonNL/orca/orchestrator/outbound/api"
	"github.com/SanteonNL/orca/orchestrator/outbound/web/assets"
	"github.com/SanteonNL/orca/orchestrator/rest"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
)

type Service struct {
	ExchangeManager *outbound.ExchangeManager
}

func (h Service) Start(listenAddress string, apiListenAddress string) error {
	httpHandler := http.NewServeMux()
	httpHandler.HandleFunc("GET /exchange/{exchangeID}/callback", func(response http.ResponseWriter, request *http.Request) {
		// The user lands on this "page" when the data exchange is completed.
		exchangeID := request.PathValue("exchangeID")
		result, err := h.ExchangeManager.HandleExchangeCallback(exchangeID)
		// TODO: This now just shows the result (or an error), which should be posted back to the EPD/Viewer (or be retrieved by it).
		if err != nil {
			http.Error(response, fmt.Sprintf("error completing data exchange: %v", err), http.StatusBadGateway)
			return
		}
		rest.RespondJSON(response, http.StatusOK, result)
	})
	// Demo-purpose endpoints
	// TODO: Remove these when not necessary any more
	httpHandler.Handle("GET /", http.FileServerFS(assets.FS))
	httpHandler.HandleFunc("POST /demo-exchange", func(writer http.ResponseWriter, request *http.Request) {
		identifierSystem := request.FormValue("system")
		identifierValue := request.FormValue("value")
		if identifierSystem == "" || identifierValue == "" {
			http.Error(writer, "system and value are required", http.StatusBadRequest)
			return
		}
		// Call "own" StartExchange() API
		requestBody, _ := json.Marshal(api.StartExchangeRequest{
			Patient: fhir.Identifier{
				System: &identifierSystem,
				Value:  &identifierValue,
			},
		})
		httpRequest, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/exchange", apiListenAddress), bytes.NewReader(requestBody))
		httpRequest.Header.Set("Content-Type", "application/json")
		httpResponse, err := http.DefaultClient.Do(httpRequest)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if httpResponse.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(httpResponse.Body)
			http.Error(writer, fmt.Sprintf("unexpected status code: %d\n%s", httpResponse.StatusCode, string(data)), http.StatusInternalServerError)
			return
		}
		var response api.StartExchangeResponse
		if err = rest.UnmarshalJSONResponseBody(httpResponse.Body, &response); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, response.RedirectURI, http.StatusFound)
	})
	err := http.ListenAndServe(listenAddress, httpHandler)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
