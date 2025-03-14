package dataview

//TODO: The name should clearly state the type of data that is being viewed. It's POST-enrolment data

import (
	"net/http"
	"net/url"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
)

func New(config Config, baseURL string) *Service {
	return &Service{config: config, baseURL: baseURL}
}

type Service struct {
	config  Config
	baseURL string
}

func (s *Service) EhrFhirProxy() coolfhir.HttpProxy {
	log.Debug().Msg("Creating DataView EHR FHIR proxy with url: " + s.config.FhirUrl)
	targetFhirBaseUrl, _ := url.Parse(s.config.FhirUrl)
	const proxyBasePath = "/cpc/fhir"
	rewriteUrl, _ := url.Parse(s.baseURL)
	rewriteUrl = rewriteUrl.JoinPath(proxyBasePath)
	result := coolfhir.NewProxy("App->EHR (DataView)", targetFhirBaseUrl, proxyBasePath, rewriteUrl, http.RoundTripper(http.DefaultTransport), false, false)
	return result
}
