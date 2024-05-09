package rest

import (
	"encoding/json"
	"errors"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
)

func RespondWithAPIError(response http.ResponseWriter, err error) {
	type apiError struct {
		Error  string      `json:"error"`
		Result interface{} `json:"result,omitempty"`
	}
	var remoteAPIError RemoteAPIError
	if errors.As(err, &remoteAPIError) {
		RespondJSON(response, http.StatusBadRequest, apiError{Error: remoteAPIError.Error(), Result: remoteAPIError.Result})
		return
	}
	RespondJSON(response, http.StatusBadRequest, apiError{Error: err.Error()})
}

func RespondJSON(response http.ResponseWriter, statusCode int, object interface{}) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(statusCode)
	if err := json.NewEncoder(response).Encode(object); err != nil {
		log.Info().Err(err).Msg("Failed to write HTTP response")
	}
}

func UnmarshalJSONRequestBody(request *http.Request, target interface{}) error {
	data, err := io.ReadAll(io.LimitReader(request.Body, 1024*1024)) // 1mb sounds sane to prevent accidental DoS
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func UnmarshalJSONResponseBody(body io.ReadCloser, target interface{}) error {
	data, err := io.ReadAll(io.LimitReader(body, 1024*1024)) // 10mb sounds sane to prevent accidental DoS
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
