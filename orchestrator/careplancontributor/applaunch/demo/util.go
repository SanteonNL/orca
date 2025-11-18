package demo

import (
	"fmt"
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
		http.Error(response, fmt.Sprintf("query parameter [%s] is expected but missing a value", key), http.StatusBadRequest)
		return "", false
	}
	return value, true
}
