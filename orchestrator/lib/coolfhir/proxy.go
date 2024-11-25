package coolfhir

import (
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir/pipeline"
	"github.com/rs/zerolog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// NewProxy creates a new FHIR reverse proxy that forwards requests to an upstream FHIR server.
// It takes the following arguments:
// - upstreamBaseUrl: the FHIR base URL of the upstream FHIR server to which HTTP requests are forwarded, e.g. http://upstream:8080/fhir
// - proxyBasePath: the base path of the proxy server, e.g. http://example.com/fhir. It is used to rewrite the request URL.
// - rewriteUrl: the base URL of the proxy server, e.g. http://example.com/fhir. It is used to rewrite URLs in the HTTP response.
// proxyBasePath and rewriteUrl might differ if the proxy server is behind a reverse proxy, which binds to application to a different path.
// E.g.:
//   - if the proxy is on /fhir, and the reverse proxy binds to /, then proxyBasePath = /fhir and rewriteUrl = /.
//   - if the proxy is on /, and the reverse proxy binds to /fhir, then proxyBasePath = / and rewriteUrl = /fhir.
//   - if the proxy is on /fhir, and the reverse proxy binds to /app/fhir, then proxyBasePath = /fhir and rewriteUrl = /app/fhir.
func NewProxy(logger zerolog.Logger, upstreamBaseUrl *url.URL, proxyBasePath string, rewriteUrl *url.URL, transport http.RoundTripper) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL = upstreamBaseUrl.JoinPath("/", strings.TrimPrefix(r.In.URL.Path, proxyBasePath))
			r.Out.URL.RawQuery = r.In.URL.RawQuery
			r.Out.Host = upstreamBaseUrl.Host
		},
		Transport: sanitizingRoundTripper{
			next: loggingRoundTripper{
				logger: &logger,
				next:   transport,
			},
		},
		ModifyResponse: func(response *http.Response) error {
			upstreamUrl := upstreamBaseUrl.String()
			proxyUrl := rewriteUrl.String()
			return pipeline.New().
				AppendResponseTransformer(pipeline.ResponseBodyRewriter{Old: []byte(upstreamUrl), New: []byte(proxyUrl)}).
				AppendResponseTransformer(pipeline.ResponseHeaderRewriter{Old: upstreamUrl, New: proxyUrl}).
				Do(response, response.Body)
		},
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			logger.Warn().Err(err).Msgf("FHIR request failed (url=%s)", request.URL.String())
			SendResponse(writer, http.StatusBadGateway, ErrorWithCode{
				Message:    "FHIR request failed: " + err.Error(),
				StatusCode: http.StatusBadGateway,
			})
		},
	}
}

var _ http.RoundTripper = &sanitizingRoundTripper{}

type sanitizingRoundTripper struct {
	next http.RoundTripper
}

func (s sanitizingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	// Header sanitizing is loosely inspired by:
	// - https://www.rfc-editor.org/rfc/rfc7231
	// - https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/header_sanitizing
	for name, _ := range request.Header {
		nameLC := strings.ToLower(name)
		if strings.HasPrefix(nameLC, "x-") ||
			nameLC == "referer" ||
			nameLC == "cookie" ||
			nameLC == "user-agent" ||
			nameLC == "authorization" {
			request.Header.Del(name)
		}
	}
	return s.next.RoundTrip(request)
}

var _ http.RoundTripper = &loggingRoundTripper{}

type loggingRoundTripper struct {
	logger *zerolog.Logger
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
