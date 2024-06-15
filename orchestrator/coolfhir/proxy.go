package coolfhir

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
