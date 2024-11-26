package coolfhir

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir/pipeline"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HttpProxy is an interface for a simple HTTP proxy that forwards requests to an upstream server.
// It's there so NewProxy can maintain compatibility with httputil.ReverseProxy
type HttpProxy interface {
	ServeHTTP(writer http.ResponseWriter, request *http.Request)
}

var _ HttpProxy = &fhirClientProxy{}

type fhirClientProxy struct {
	client          fhirclient.Client
	proxyBasePath   string
	proxyBaseUrl    *url.URL
	upstreamBaseUrl *url.URL
}

func (f fhirClientProxy) ServeHTTP(httpResponseWriter http.ResponseWriter, request *http.Request) {
	outRequestUrl := f.upstreamBaseUrl.JoinPath(strings.TrimPrefix(request.URL.Path, f.proxyBasePath))
	var responseStatusCode int
	var headers fhirclient.Headers
	params := []fhirclient.Option{
		fhirclient.RequestHeaders(f.sanitizeRequestHeaders(request.Header)),
		fhirclient.ResponseHeaders(&headers),
		fhirclient.ResponseStatusCode(&responseStatusCode),
		fhirclient.AtUrl(outRequestUrl),
	}
	for key, values := range request.URL.Query() {
		for _, value := range values {
			params = append(params, fhirclient.QueryParam(key, value))
		}
	}
	// Read the request body, making sure it's valid JSON
	var err error
	var requestResource map[string]interface{}
	if request.Body != nil {
		// LimitReader 10mb to prevent DoS attacks
		requestData, err := io.ReadAll(io.LimitReader(request.Body, 10*1024*1024+1))
		if len(requestData) > 10*1024*1024 {
			WriteOperationOutcomeFromError(fhirclient.OperationOutcomeError{
				OperationOutcome: fhir.OperationOutcome{
					Issue: []fhir.OperationOutcomeIssue{
						{
							Severity:    fhir.IssueSeverityError,
							Code:        fhir.IssueTypeStructure,
							Diagnostics: to.Ptr("Request body is too large"),
						},
					},
				},
				HttpStatusCode: http.StatusRequestEntityTooLarge,
			}, "FHIR request", httpResponseWriter)
			return
		}
		if err != nil {
			WriteOperationOutcomeFromError(fhirclient.OperationOutcomeError{
				OperationOutcome: fhir.OperationOutcome{
					Issue: []fhir.OperationOutcomeIssue{
						{
							Severity:    fhir.IssueSeverityError,
							Code:        fhir.IssueTypeStructure,
							Diagnostics: to.Ptr("Couldn't read request body: " + err.Error()),
						},
					},
				},
				HttpStatusCode: http.StatusBadRequest,
			}, "FHIR request", httpResponseWriter)
			return
		}
		if len(requestData) > 0 {
			requestResource = make(map[string]interface{})
			if err := json.Unmarshal(requestData, &requestResource); err != nil {
				WriteOperationOutcomeFromError(fhirclient.OperationOutcomeError{
					OperationOutcome: fhir.OperationOutcome{
						Issue: []fhir.OperationOutcomeIssue{
							{
								Severity:    fhir.IssueSeverityError,
								Code:        fhir.IssueTypeStructure,
								Diagnostics: to.Ptr("Request body isn't valid JSON: " + err.Error()),
							},
						},
					},
					HttpStatusCode: http.StatusBadRequest,
				}, "FHIR request", httpResponseWriter)
				return
			}
		}
	}
	if requestResource == nil && (request.Method == http.MethodPost || request.Method == http.MethodPut) {
		WriteOperationOutcomeFromError(fhirclient.OperationOutcomeError{
			OperationOutcome: fhir.OperationOutcome{
				Issue: []fhir.OperationOutcomeIssue{
					{
						Severity:    fhir.IssueSeverityError,
						Code:        fhir.IssueTypeStructure,
						Diagnostics: to.Ptr("Request body is required for " + request.Method + " requests"),
					},
				},
			},
			HttpStatusCode: http.StatusBadRequest,
		}, "FHIR request", httpResponseWriter)
		return
	}

	responseResource := make(map[string]interface{})

	// Execute the request
	switch request.Method {
	case http.MethodGet:
		err = f.client.ReadWithContext(request.Context(), outRequestUrl.Path, &responseResource, params...)
	case http.MethodPost:
		err = f.client.CreateWithContext(request.Context(), requestResource, &responseResource, params...)
	case http.MethodPut:
		err = f.client.UpdateWithContext(request.Context(), outRequestUrl.Path, requestResource, &responseResource, params...)
	default:
		SendResponse(httpResponseWriter, http.StatusMethodNotAllowed, BadRequest("Method not allowed: %s", request.Method))
		return
	}

	if err != nil {
		// Make sure we always return a FHIR OperationOutcome in case of an error
		if !errors.As(err, &fhirclient.OperationOutcomeError{}) {
			err = fhirclient.OperationOutcomeError{
				OperationOutcome: fhir.OperationOutcome{
					Issue: []fhir.OperationOutcomeIssue{
						{
							Severity:    fhir.IssueSeverityError,
							Code:        fhir.IssueTypeProcessing,
							Diagnostics: to.Ptr(err.Error()),
						},
					},
				},
				HttpStatusCode: responseStatusCode,
			}
		}
		WriteOperationOutcomeFromError(err, "FHIR request", httpResponseWriter)
		return
	}
	upstreamUrl := f.upstreamBaseUrl.String()
	proxyUrl := f.proxyBaseUrl.String()
	pipeline.New().
		AppendResponseTransformer(pipeline.ResponseBodyRewriter{Old: []byte(upstreamUrl), New: []byte(proxyUrl)}).
		AppendResponseTransformer(pipeline.ResponseHeaderRewriter{Old: upstreamUrl, New: proxyUrl}).
		DoAndWrite(httpResponseWriter, responseResource, responseStatusCode)
}

func (f fhirClientProxy) sanitizeRequestHeaders(header http.Header) http.Header {
	result := make(http.Header)
	// Header sanitizing is loosely inspired by:
	// - https://www.rfc-editor.org/rfc/rfc7231
	// - https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/header_sanitizing
	for name, values := range header {
		nameLC := strings.ToLower(name)
		if strings.HasPrefix(nameLC, "x-") ||
			nameLC == "referer" ||
			nameLC == "cookie" ||
			nameLC == "user-agent" ||
			nameLC == "authorization" {
			continue
		}
		result[name] = values
	}
	return result
}

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
func NewProxy(name string, logger zerolog.Logger, upstreamBaseUrl *url.URL, proxyBasePath string, rewriteUrl *url.URL, transport http.RoundTripper) HttpProxy {
	httpClient := &http.Client{
		Transport: &loggingRoundTripper{
			name:   name,
			logger: &logger,
			next:   transport,
		},
	}
	return &fhirClientProxy{
		client:          fhirclient.New(upstreamBaseUrl, httpClient, nil),
		proxyBasePath:   proxyBasePath,
		proxyBaseUrl:    rewriteUrl,
		upstreamBaseUrl: upstreamBaseUrl,
	}
}

var _ http.RoundTripper = &loggingRoundTripper{}

type loggingRoundTripper struct {
	name   string
	logger *zerolog.Logger
	next   http.RoundTripper
}

func (l loggingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	l.logger.Info().Ctx(request.Context()).Msgf("%s request: %s %s", l.name, request.Method, request.URL.String())
	if l.logger.Debug().Ctx(request.Context()).Enabled() {
		var headers []string
		for key, values := range request.Header {
			headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
		}
		l.logger.Debug().Ctx(request.Context()).Msgf("%s request headers: %s", l.name, strings.Join(headers, ", "))
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
		l.logger.Trace().Ctx(request.Context()).Msgf("%s request body: %s", l.name, string(requestBody))
		request.Body = io.NopCloser(bytes.NewReader(requestBody))
	}
	response, err := l.next.RoundTrip(request)
	if err != nil {
		l.logger.Warn().Err(err).Msgf("%s request failed (url=%s)", l.name, request.URL.String())
	} else {
		if l.logger.Debug().Ctx(request.Context()).Enabled() {
			var headers []string
			for key, values := range response.Header {
				headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
			}
			l.logger.Debug().Ctx(request.Context()).Msgf("%s response: %s, headers: %s", l.name, response.Status, strings.Join(headers, ", "))
		}
		if l.logger.Trace().Ctx(request.Context()).Enabled() {
			responseBody, err := io.ReadAll(response.Body)
			if err != nil {
				return nil, err
			}
			l.logger.Trace().Ctx(request.Context()).Msgf("%s response body: %s", l.name, string(responseBody))
			response.Body = io.NopCloser(bytes.NewReader(responseBody))
		}
	}
	return response, err
}
