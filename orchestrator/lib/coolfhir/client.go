package coolfhir

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/rs/zerolog/log"
	"net/http"
)

type ClientCreator func(properties map[string]string) fhirclient.Client

var ClientFactories = map[string]ClientCreator{}

func Config() *fhirclient.Config {
	return &fhirclient.Config{
		Non2xxStatusHandler: func(response *http.Response, responseBody []byte) {
			log.Debug().Msgf("Non-2xx status code from FHIR server (code=%d, url=%s), content: %s", response.StatusCode, response.Request.URL, string(responseBody))
		},
	}
}
