package coolfhir

import (
	"bytes"
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

type HttpProxy interface {
	ServeHTTP(writer http.ResponseWriter, request *http.Request)
}

func responseStatus(status *int) fhirclient.PostRequestOption {
	return func(_ fhirclient.Client, r *http.Response) error {
		*status = r.StatusCode
		return nil
	}
}

var _ HttpProxy = &FhirClientProxy{}

type FhirClientProxy struct {
	client          fhirclient.Client
	proxyBasePath   string
	proxyBaseUrl    *url.URL
	upstreamBaseUrl *url.URL
}

func (f FhirClientProxy) ServeHTTP(httpResponseWriter http.ResponseWriter, request *http.Request) {
	outRequestUrl := f.upstreamBaseUrl.JoinPath(strings.TrimPrefix(request.URL.Path, f.proxyBasePath))
	var responseStatusCode int
	var headers fhirclient.Headers
	params := []fhirclient.Option{
		fhirclient.ResponseHeaders(&headers),
		responseStatus(&responseStatusCode),
		fhirclient.AtUrl(outRequestUrl),
	}
	for key, values := range request.URL.Query() {
		for _, value := range values {
			params = append(params, fhirclient.QueryParam(key, value))
		}
	}
	// LimitReader 10mb to prevent DoS attacks
	requestData, err := io.ReadAll(io.LimitReader(request.Body, 10*1024*1024))
	if err != nil {
		SendResponse(httpResponseWriter, http.StatusBadRequest, &ErrorWithCode{
			Message:    "Failed to read request body: " + err.Error(),
			StatusCode: http.StatusBadRequest,
		})
		return
	}
	var responseData []byte
	switch request.Method {
	case http.MethodGet:
		err = f.client.ReadWithContext(request.Context(), outRequestUrl.Path, &responseData, params...)
	case http.MethodPost:
		err = f.client.CreateWithContext(request.Context(), requestData, &responseData, params...)
	case http.MethodPut:
		err = f.client.UpdateWithContext(request.Context(), outRequestUrl.Path, requestData, &responseData, params...)
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
		DoAndWrite(httpResponseWriter, responseData, responseStatusCode)
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
	return &FhirClientProxy{
		client:          fhirclient.New(upstreamBaseUrl, httpClient, nil),
		proxyBasePath:   proxyBasePath,
		proxyBaseUrl:    rewriteUrl,
		upstreamBaseUrl: upstreamBaseUrl,
	}

	//return &httputil.ReverseProxy{
	//	Rewrite: func(r *httputil.ProxyRequest) {
	//		r.Out.URL = upstreamBaseUrl.JoinPath(strings.TrimPrefix(r.In.URL.Path, proxyBasePath))
	//		r.Out.URL.RawQuery = r.In.URL.RawQuery
	//		r.Out.Host = "" // upstreamBaseUrl.Host
	//	},
	//	Transport: sanitizingRoundTripper{
	//		next: loggingRoundTripper{
	//			logger: &logger,
	//			next:   transport,
	//			name:   name,
	//		},
	//	},
	//	ModifyResponse: func(response *http.Response) error {
	//		upstreamUrl := upstreamBaseUrl.String()
	//		proxyUrl := rewriteUrl.String()
	//		return pipeline.New().
	//			AppendResponseTransformer(pipeline.ResponseBodyRewriter{Old: []byte(upstreamUrl), New: []byte(proxyUrl)}).
	//			AppendResponseTransformer(pipeline.ResponseHeaderRewriter{Old: upstreamUrl, New: proxyUrl}).
	//			Do(response, response.Body)
	//	},
	//	ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
	//		logger.Warn().Err(err).Msgf("%s request failed (url=%s)", name, request.URL.String())
	//		SendResponse(writer, http.StatusBadGateway, &ErrorWithCode{
	//			Message:    "FHIR request failed: " + err.Error(),
	//			StatusCode: http.StatusBadGateway,
	//		})
	//	},
	//}
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
	// TODO: Do we need this, maybe it was added by curl?
	if request.Method == http.MethodGet {
		request.Header.Del("Content-Length")
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
