package coolfhir

import (
	"fmt"
	"github.com/rs/zerolog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func NewProxy(logger zerolog.Logger, targetFHIRBaseURL *url.URL, proxyBasePath string, transport http.RoundTripper) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL = targetFHIRBaseURL.JoinPath("/", strings.TrimPrefix(r.In.URL.Path, proxyBasePath))
			r.Out.Host = targetFHIRBaseURL.Host
		},
		Transport: loggingRoundTripper{
			next: transport,
		},
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			logger.Warn().Err(err).Msgf("FHIR request failed (url=%s)", request.URL.String())
			http.Error(writer, "FHIR request failed: "+err.Error(), http.StatusBadGateway)
		},
	}
}

var _ http.RoundTripper = &loggingRoundTripper{}

type loggingRoundTripper struct {
	logger zerolog.Logger
	next   http.RoundTripper
}

func (l loggingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	l.logger.Info().Msgf("Proxying FHIR request: %s %s", request.Method, request.URL.String())
	if l.logger.Debug().Enabled() {
		var headers []string
		for key, values := range request.Header {
			headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
		}
		l.logger.Debug().Msgf("Proxy request headers: %s", strings.Join(headers, ", "))
	}
	response, err := l.next.RoundTrip(request)
	if err != nil {
		l.logger.Warn().Err(err).Msgf("Proxied FHIR request failed (url=%s)", request.URL.String())
	} else {
		if l.logger.Debug().Enabled() {
			l.logger.Debug().Msgf("Proxied FHIR request response: %s", response.Status)
			var headers []string
			for key, values := range response.Header {
				headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
			}
			l.logger.Debug().Msgf("Proxy response headers: %s", strings.Join(headers, ", "))
		}
	}
	return response, err
}

//func CreateProxy(fhirBaseURL *url.URL) {
//	result := &httputil.ReverseProxy{
//		Rewrite: func(r *httputil.ProxyRequest) {
//			r.SetURL(fhirBaseURL)
//			cleanHeaders(r.Out.Header)
//		},
//	}.ServeHTTP
//	result.Transport = loggingTransportDecorator{
//		RoundTripper: fhirHTTPClient.Transport,
//	}
//	result.ErrorHandler = func(responseWriter http.ResponseWriter, request *http.Request, err error) {
//		log.Warn().Err(err).Msgf("Proxy error: %s", sanitizeRequestURL(request.URL).String())
//		responseWriter.Header().Add("Content-Type", "application/fhir+json")
//		responseWriter.WriteHeader(http.StatusBadGateway)
//		diagnostics := "The system tried to proxy the FHIR operation, but an error occurred."
//		data, _ := json.Marshal(fhir.OperationOutcome{
//			Issue: []fhir.OperationOutcomeIssue{
//				{
//					Severity:    fhir.IssueSeverityError,
//					Diagnostics: &diagnostics,
//				},
//			},
//		})
//		_, _ = responseWriter.Write(data)
//	}
//}
//
//func cleanHeaders(header http.Header) {
//	for name, _ := range header {
//		switch name {
//		case "Content-Type":
//			continue
//		case "Accept":
//			continue
//		case "Accept-Encoding":
//			continue
//		case "User-Agent":
//			continue
//		case "X-Request-Id":
//			// useful for tracing
//			continue
//		default:
//			header.Del(name)
//		}
//	}
//}
//
//type loggingTransportDecorator struct {
//	RoundTripper http.RoundTripper
//}
//
//func (d loggingTransportDecorator) RoundTrip(request *http.Request) (*http.Response, error) {
//	response, err := d.RoundTripper.RoundTrip(request)
//	if err != nil {
//		log.Warn().Msgf("Proxy request failed: %s", sanitizeRequestURL(request.URL).String())
//	} else {
//		log.Info().Msgf("Proxied request: %s", sanitizeRequestURL(request.URL).String())
//	}
//	return response, err
//}
//
//func sanitizeRequestURL(requestURL *url.URL) *url.URL {
//	// Query might contain PII (e.g., social security number), so do not log it.
//	requestURLWithoutQuery := *requestURL
//	requestURLWithoutQuery.RawQuery = ""
//	return &requestURLWithoutQuery
//}
