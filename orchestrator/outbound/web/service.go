package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/outbound"
	"github.com/SanteonNL/orca/orchestrator/outbound/api"
	"github.com/SanteonNL/orca/orchestrator/outbound/web/assets"
	"io"
	"net/http"
)

type Service struct {
	ExchangeManager *outbound.ExchangeManager
}

func (h Service) Start(listenAddress string, apiListenAddress string) error {
	httpHandler := http.NewServeMux()
	httpHandler.HandleFunc("GET /exchange/{exchangeID}/callback", func(responseWriter http.ResponseWriter, request *http.Request) {
		// The user lands on this "page" when the data exchange is completed.
		exchangeID := request.PathValue("exchangeID")
		err := h.ExchangeManager.HandleExchangeCallback(exchangeID)
		// TODO: This now just shows the result (or an error), which should be posted back to the EPD/Viewer (or be retrieved by it).
		if err != nil {
			http.Error(responseWriter, fmt.Sprintf("error completing data exchange: %v", err), http.StatusBadGateway)
			return
		}
		data, err := h.retrieveExchangeResult(apiListenAddress, exchangeID)
		if err != nil {
			http.Error(responseWriter, "Exchange result retrieval error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write(data)
	})
	// Demo-purpose endpoints
	// TODO: Remove these when not necessary any more
	httpHandler.Handle("GET /", http.FileServerFS(assets.FS))
	httpHandler.HandleFunc("POST /demo-exchange", func(responseWriter http.ResponseWriter, request *http.Request) {
		fhirOperationPath := request.FormValue("fhirOperationPath")
		// Call "own" StartExchange() API
		requestBody, _ := json.Marshal(api.StartExchangeJSONRequestBody{
			FhirOperationPath: fhirOperationPath,
			Oauth2Scope:       "homemonitoring",
		})
		httpRequest, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/exchange", apiListenAddress), bytes.NewReader(requestBody))
		httpRequest.Header.Set("Content-Type", "application/json")
		httpResponse, err := http.DefaultClient.Do(httpRequest)
		if err != nil {
			http.Error(responseWriter, "Exchange start error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		data, _ := io.ReadAll(httpResponse.Body)
		switch httpResponse.StatusCode {
		case http.StatusOK:
			// Data already retrieved using cached access tokens
			var response api.StartExchange200JSONResponse
			if err = json.Unmarshal(data, &response); err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
				return
			}
			// Get result, simply send data back to client
			data, err := h.retrieveExchangeResult(apiListenAddress, response.ExchangeId)
			if err != nil {
				http.Error(responseWriter, "Exchange result retrieval error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			responseWriter.Header().Set("Content-Type", "application/json")
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write(data)
		case http.StatusCreated:
			// User needs to be authenticated at the remote party(s)
			var response api.StartExchange201JSONResponse
			if err = json.Unmarshal(data, &response); err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(responseWriter, request, response.RedirectUrl, http.StatusFound)
		default:
			http.Error(responseWriter, fmt.Sprintf("Failed to start exchange, unexpected status code: %d\nresponse data: %s", httpResponse.StatusCode, string(data)), http.StatusInternalServerError)
		}
	})
	err := http.ListenAndServe(listenAddress, httpHandler)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (h Service) retrieveExchangeResult(listenAddress string, exchangeID string) ([]byte, error) {
	httpResponse, err := http.Get(fmt.Sprintf("http://%s/exchange/%s/result", listenAddress, exchangeID))
	defer httpResponse.Body.Close()
	if err != nil {
		return nil, err
	}
	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to retrieve exchange result, unexpected status code: %d", httpResponse.StatusCode)
	}
	return io.ReadAll(httpResponse.Body)
}
