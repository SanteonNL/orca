package coolfhir

import (
	"bytes"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir/pipeline"
	"github.com/rs/zerolog"
	"io"
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
func NewProxy(name string, logger zerolog.Logger, upstreamBaseUrl *url.URL, proxyBasePath string, rewriteUrl *url.URL, transport http.RoundTripper) *httputil.ReverseProxy {
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
			logger.Warn().Err(err).Msgf("%s proxy FHIR request failed (url=%s)", name, request.URL.String())
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
	name   string
	logger *zerolog.Logger
	next   http.RoundTripper
}

func (l loggingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	l.logger.Info().Ctx(request.Context()).Msgf("%s proxying FHIR request: %s %s", l.name, request.Method, request.URL.String())
	if l.logger.Debug().Ctx(request.Context()).Enabled() {
		var headers []string
		for key, values := range request.Header {
			headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
		}
		l.logger.Debug().Ctx(request.Context()).Msgf("%s proxy FHIR request headers: %s", l.name, strings.Join(headers, ", "))
	}
	if l.logger.Trace().Ctx(request.Context()).Enabled() {
		var requestBody []byte
		var err error
		if request.Body != nil {
			requestBody, err = io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
		}
		l.logger.Trace().Ctx(request.Context()).Msgf("%s proxy FHIR request body: %s", l.name, string(requestBody))
		request.Body = io.NopCloser(bytes.NewReader(requestBody))
	}
	response, err := l.next.RoundTrip(request)
	if err != nil {
		l.logger.Warn().Err(err).Msgf("%s proxied FHIR request failed (url=%s)", l.name, request.URL.String())
	} else {
		if l.logger.Debug().Ctx(request.Context()).Enabled() {
			var headers []string
			for key, values := range response.Header {
				headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
			}
			l.logger.Debug().Ctx(request.Context()).Msgf("%s proxied FHIR response: %s, headers: %s", l.name, response.Status, strings.Join(headers, ", "))
		}
		if l.logger.Trace().Ctx(request.Context()).Enabled() {
			responseBody, err := io.ReadAll(response.Body)
			if err != nil {
				return nil, err
			}
			l.logger.Trace().Ctx(request.Context()).Msgf("%s proxied FHIR response body: %s", l.name, string(responseBody))
			response.Body = io.NopCloser(bytes.NewReader(responseBody))
		}
	}
	return response, err
}
