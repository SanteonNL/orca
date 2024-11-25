package coolfhir

import "net/url"

// NoopUrlLoggerSanitizer is a URL logger sanitizer that does nothing.
func NoopUrlLoggerSanitizer(in *url.URL) *url.URL {
	return in
}

// FhirUrlLoggerSanitizer is a URL logger sanitizer that masks certain query parameters.
func FhirUrlLoggerSanitizer(in *url.URL) *url.URL {
	result := *in
	q := url.Values{}
	for name, values := range in.Query() {
		for _, value := range values {
			switch name {
			case "_include":
				// Don't mask
				q.Add(name, value)
			default:
				// Mask others
				q.Add(name, "****")
			}

		}
	}
	result.RawQuery = q.Encode()
	return &result
}
