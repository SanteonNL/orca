package demo

import (
	"log/slog"
	"net/http"
)

func getQueryParams(response http.ResponseWriter, request *http.Request, keys ...string) (map[string]string, bool) {
	results := map[string]string{}
	for _, key := range keys {
		value, b := getQueryParam(response, request, key)
		if !b {
			return nil, false
		}
		results[key] = value
	}
	return results, true
}

func getQueryParam(response http.ResponseWriter, request *http.Request, key string) (string, bool) {
	value := request.URL.Query().Get(key)
	if value == "" {
		slog.ErrorContext(request.Context(), "missing query parameter", slog.String("key", key))
		http.Error(response, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return "", false
	}
	return value, true
}
