package coolfhir

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/rs/zerolog/log"
	"net/http"
)

type ClientCreator func(properties map[string]string) fhirclient.Client

var ClientFactories = map[string]ClientCreator{}

func Config() *fhirclient.Config {
	config := fhirclient.DefaultConfig()
	config.Non2xxStatusHandler = func(response *http.Response, responseBody []byte) {
		log.Debug().Msgf("Non-2xx status code from FHIR server (%s %s, status=%d), content: %s", response.Request.Method, response.Request.URL, response.StatusCode, string(responseBody))
	}
	return &config
}
