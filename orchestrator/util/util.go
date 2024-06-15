package util

import "net/http"

func GetQueryParams(response http.ResponseWriter, request *http.Request, keys ...string) (map[string]string, bool) {
	results := map[string]string{}
	for _, key := range keys {
		value, b := GetQueryParam(response, request, key)
		if !b {
			return nil, false
		}
		results[key] = value
	}
	return results, true
}

func GetQueryParam(response http.ResponseWriter, request *http.Request, key string) (string, bool) {
	value := request.URL.Query().Get(key)
	if value == "" {
		http.Error(response, "missing query parameter: "+key, http.StatusBadRequest)
		return "", false
	}
	return value, true
}
