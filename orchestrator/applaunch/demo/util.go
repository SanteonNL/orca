package demo

import "net/http"

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
		http.Error(response, "missing query parameter: "+key, http.StatusBadRequest)
		return "", false
	}
	return value, true
}
