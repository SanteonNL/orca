package coolfhir

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir/pipeline"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var tracer = baseotel.Tracer("proxy")

// HttpProxy is an interface for a simple HTTP proxy that forwards requests to an upstream server.
// It's there so NewProxy can maintain compatibility with httputil.ReverseProxy
type HttpProxy interface {
	ServeHTTP(writer http.ResponseWriter, request *http.Request)
}

var _ HttpProxy = &FHIRClientProxy{}

type FHIRClientProxy struct {
	client          fhirclient.Client
	proxyBasePath   string
	proxyBaseUrl    *url.URL
	upstreamBaseUrl *url.URL
	allowCaching    bool
	setMetaSource   bool
	// HTTPRequestModifier allows modification of the HTTP request before it is proxied to the upstream server.
	// It can be used to build smarter proxies (e.g. changing HTTP methods or other semantics).
	// It must either return the modified (or the original request), or an error.
	HTTPRequestModifier func(*http.Request) (*http.Request, error)
}

func (f *FHIRClientProxy) ServeHTTP(httpResponseWriter http.ResponseWriter, request *http.Request) {
	ctx, span := tracer.Start(
		request.Context(),
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.HTTPMethod, request.Method),
			attribute.String(otel.HTTPURL, request.URL.String()),
			attribute.String("proxy.base_path", f.proxyBasePath),
		),
	)
	defer span.End()

	request = request.WithContext(ctx)

	if f.HTTPRequestModifier != nil {
		var err error
		if request, err = f.HTTPRequestModifier(request); err != nil {
			WriteOperationOutcomeFromError(ctx, otel.Error(span, err), "FHIR request", httpResponseWriter)
			return
		}
	}

	outRequestUrl := f.upstreamBaseUrl.JoinPath(strings.TrimPrefix(request.URL.Path, f.proxyBasePath))
	if strings.HasSuffix(request.URL.Path, "/") &&
		!strings.HasSuffix(outRequestUrl.Path, "/") {
		// Request was with trailing slash, but got removed by the path construction above, so re-add it.
		outRequestUrl.Path += "/"
	}
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
	var requestData []byte
	if request.Body != nil {
		// LimitReader 10mb to prevent DoS attacks
		requestData, err = io.ReadAll(io.LimitReader(request.Body, 10*1024*1024+1))
		if len(requestData) > 10*1024*1024 {
			WriteOperationOutcomeFromError(ctx, otel.Error(span, fhirclient.OperationOutcomeError{
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
			}), "FHIR request", httpResponseWriter)
			return
		}
		if err != nil {
			WriteOperationOutcomeFromError(ctx, otel.Error(span, fhirclient.OperationOutcomeError{
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
			}), "FHIR request", httpResponseWriter)
			return
		}
		if len(requestData) > 0 && !strings.HasSuffix(request.URL.Path, "/_search") {
			requestResource = make(map[string]interface{})
			if err := json.Unmarshal(requestData, &requestResource); err != nil {
				WriteOperationOutcomeFromError(ctx, otel.Error(span, fhirclient.OperationOutcomeError{
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
				}), "FHIR request", httpResponseWriter)
				return
			}
		}
	}
	if requestResource == nil && (request.Method == http.MethodPost || request.Method == http.MethodPut) && !strings.HasSuffix(request.URL.Path, "/_search") {
		WriteOperationOutcomeFromError(ctx, otel.Error(span, fhirclient.OperationOutcomeError{
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
		}), "FHIR request", httpResponseWriter)
		return
	}

	responseResource := make(map[string]interface{})

	// Execute the request
	switch request.Method {
	case http.MethodGet:
		err = f.client.ReadWithContext(ctx, outRequestUrl.Path, &responseResource, params...)
	case http.MethodPost:
		if strings.HasSuffix(request.URL.Path, "/_search") {
			var values url.Values
			values, err = url.ParseQuery(string(requestData))
			if err == nil {
				err = f.client.SearchWithContext(ctx, strings.TrimSuffix(outRequestUrl.Path, "/_search"), values, &responseResource, params...)
			}
		} else {
			err = f.client.CreateWithContext(ctx, requestResource, &responseResource, params...)
		}
	case http.MethodPut:
		err = f.client.UpdateWithContext(ctx, outRequestUrl.Path, requestResource, &responseResource, params...)
	case http.MethodDelete:
		err = f.client.DeleteWithContext(ctx, outRequestUrl.Path, params...)
	default:
		SendResponse(httpResponseWriter, http.StatusMethodNotAllowed, BadRequest("Method not allowed: %s", request.Method))
		return
	}

	if err != nil {
		// Make sure we always return a FHIR OperationOutcome in case of an error
		if !errors.As(err, &fhirclient.OperationOutcomeError{}) {
			// Don't return a status 200 OK from the upstream server if processing the result failed. E.g., server returned 200 OK with non-JSON.
			if responseStatusCode >= 200 && responseStatusCode <= 299 {
				responseStatusCode = http.StatusBadGateway
			}
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
		WriteOperationOutcomeFromError(ctx, otel.Error(span, err), "FHIR request", httpResponseWriter)
		return
	}
	upstreamUrl := f.upstreamBaseUrl.String()
	proxyUrl := f.proxyBaseUrl.String()
	pipe := pipeline.New().
		AppendResponseTransformer(pipeline.ResponseBodyRewriter{Old: []byte(upstreamUrl), New: []byte(proxyUrl)}).
		AppendResponseTransformer(pipeline.ResponseHeaderRewriter{Old: upstreamUrl, New: proxyUrl})
	if f.allowCaching {
		pipe = pipe.AppendResponseTransformer(pipeline.ResponseHeaderSetter{
			"Cache-Control": {"must-understand, private"},
		})
	} else {
		pipe = pipe.AppendResponseTransformer(pipeline.ResponseHeaderSetter{
			"Cache-Control": {"no-store"},
		})
	}
	if f.setMetaSource && request.Method == http.MethodGet {
		// Note: only for read operations
		pipe = pipe.AppendResponseTransformer(pipeline.MetaSourceSetter{URI: outRequestUrl.String()})
	}
	pipe.DoAndWrite(ctx, tracer, httpResponseWriter, responseResource, responseStatusCode)
}

func (f *FHIRClientProxy) sanitizeRequestHeaders(header http.Header) http.Header {
	result := make(http.Header)
	// Header sanitizing is loosely inspired by:
	// - https://www.rfc-editor.org/rfc/rfc7231
	// - https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/header_sanitizing
	// - httputil.ReverseProxy: remove hop-by-hop headers
	for name, values := range header {
		nameLC := strings.ToLower(name)
		if strings.HasPrefix(nameLC, "x-") && nameLC != "x-scp-context" ||
			nameLC == "referer" ||
			nameLC == "cookie" ||
			nameLC == "user-agent" ||
			nameLC == "accept-encoding" ||
			nameLC == "authorization" ||
			nameLC == "connection" ||
			nameLC == "proxy-connection" ||
			nameLC == "keep-alive" ||
			nameLC == "proxy-authenticate" ||
			nameLC == "proxy-authorization" ||
			nameLC == "te" ||
			nameLC == "trailer" ||
			nameLC == "transfer-encoding" ||
			nameLC == "upgrade" {
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
// - allowCaching: controls Cache-Control header directives of HTTP responses.
// proxyBasePath and rewriteUrl might differ if the proxy server is behind a reverse proxy, which binds to application to a different path.
// E.g.:
//   - if the proxy is on /fhir, and the reverse proxy binds to /, then proxyBasePath = /fhir and rewriteUrl = /.
//   - if the proxy is on /, and the reverse proxy binds to /fhir, then proxyBasePath = / and rewriteUrl = /fhir.
//   - if the proxy is on /fhir, and the reverse proxy binds to /app/fhir, then proxyBasePath = /fhir and rewriteUrl = /app/fhir.
func NewProxy(name string, upstreamBaseUrl *url.URL, proxyBasePath string, rewriteUrl *url.URL,
	transport http.RoundTripper, allowCaching bool, setMetaSource bool) *FHIRClientProxy {
	httpClient := &http.Client{
		Transport: &LoggingRoundTripper{
			Name: name,
			Next: transport,
		},
	}
	return &FHIRClientProxy{
		client:          fhirclient.New(upstreamBaseUrl, httpClient, Config()),
		proxyBasePath:   proxyBasePath,
		proxyBaseUrl:    rewriteUrl,
		upstreamBaseUrl: upstreamBaseUrl,
		allowCaching:    allowCaching,
		setMetaSource:   setMetaSource,
	}
}

var _ http.RoundTripper = &LoggingRoundTripper{}

type LoggingRoundTripper struct {
	Name string
	Next http.RoundTripper
}

func (l LoggingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	ctx, span := tracer.Start(
		request.Context(),
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.HTTPMethod, request.Method),
			attribute.String(otel.HTTPURL, request.URL.String()),
		),
	)
	defer span.End()

	slog.DebugContext(ctx, "RoundTrip Request",
		slog.String("name", l.Name),
		slog.String("method", request.Method),
		slog.String(logging.FieldUrl, request.URL.String()),
	)
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		var headers []string
		for key, values := range request.Header {
			headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
		}
		var requestBody []byte
		var err error
		if request.Body != nil {
			requestBody, err = io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
		}
		slog.DebugContext(ctx, "RoundTrip Request",
			slog.String("name", l.Name),
			slog.String("headers", strings.Join(headers, ", ")),
			slog.String("body", string(requestBody)),
		)
		request.Body = io.NopCloser(bytes.NewReader(requestBody))
	}
	response, err := l.Next.RoundTrip(request)
	if err != nil {
		slog.WarnContext(ctx, "RoundTrip Request failed",
			slog.String("name", l.Name),
			slog.String(logging.FieldUrl, request.URL.String()),
			slog.String(logging.FieldError, err.Error()),
		)
	} else if slog.Default().Enabled(ctx, slog.LevelDebug) {
		var headers []string
		for key, values := range response.Header {
			headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
		}
		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		slog.DebugContext(ctx, "RoundTrip response",
			slog.String("name", l.Name),
			slog.String("status", response.Status),
			slog.String("headers", strings.Join(headers, ", ")),
			slog.String("body", string(responseBody)),
		)
		response.Body = io.NopCloser(bytes.NewReader(responseBody))
	}
	return response, err
}
