package otel

import (
	"net/url"
	"strings"
)

// Redacted is the placeholder used in place of any potentially-PII value that is
// attached to a span or log record.
const Redacted = "***"

// RedactURL returns the URL with every query-parameter VALUE replaced by [Redacted],
// while preserving the scheme, host, path, and parameter keys.
//
// FHIR search URLs routinely embed identifiers and BSNs in query values
// (e.g. "Patient?identifier=http://fhir.nl/fhir/NamingSystem/bsn|999999151"), so the
// raw query string must never be attached to a span attribute or log field. Keeping the
// keys keeps the span shape useful for debugging without leaking the values.
//
// If the URL cannot be parsed, everything after the first '?' is dropped as a safe
// fallback so no query data escapes.
func RedactURL(raw string) string {
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		if i := strings.IndexByte(raw, '?'); i >= 0 {
			return raw[:i] + "?" + Redacted
		}
		return raw
	}
	if u.RawQuery == "" {
		return u.String()
	}
	q := u.Query()
	for k := range q {
		q[k] = []string{Redacted}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// MaskBSN returns a redacted placeholder for a BSN (Dutch citizen service number).
// It returns an empty string for empty input so that "not set" stays distinguishable
// from "masked".
//
// Note: FHIR resource ids and references are GUIDs (pseudonymous, not direct PII) and are
// deliberately NOT masked, so no MaskIdentifier helper is provided.
func MaskBSN(bsn string) string {
	if bsn == "" {
		return ""
	}
	return Redacted
}
