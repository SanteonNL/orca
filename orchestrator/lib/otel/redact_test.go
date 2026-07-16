package otel

import (
	"strings"
	"testing"
)

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "no query is unchanged",
			in:   "http://fhirstore:8080/fhir/Task/123",
			want: "http://fhirstore:8080/fhir/Task/123",
		},
		{
			name: "FHIR identifier search with BSN is masked",
			in:   "http://fhirstore:8080/fhir/Patient?identifier=http://fhir.nl/fhir/NamingSystem/bsn|999999151",
			want: "http://fhirstore:8080/fhir/Patient?identifier=%2A%2A%2A",
		},
		{
			name: "multiple query params all masked, keys preserved",
			in:   "http://example.com/fhir/Patient?identifier=sys|999999151&_count=50",
			want: "http://example.com/fhir/Patient?_count=%2A%2A%2A&identifier=%2A%2A%2A",
		},
		{
			name: "path with resource id kept, query masked",
			in:   "https://host/cps/Task?patient=Patient/abc&status=active",
			want: "https://host/cps/Task?patient=%2A%2A%2A&status=%2A%2A%2A",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactURL(tt.in); got != tt.want {
				t.Fatalf("RedactURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestRedactURL_BSNNeverSurvives is a focused guard that a BSN in a query value is never
// emitted verbatim.
func TestRedactURL_BSNNeverSurvives(t *testing.T) {
	const bsn = "999999151"
	got := RedactURL("http://host/fhir/Patient?identifier=http://fhir.nl/fhir/NamingSystem/bsn|" + bsn)
	if strings.Contains(got, bsn) {
		t.Fatalf("RedactURL leaked BSN: %q", got)
	}
}

func TestMaskBSN(t *testing.T) {
	if got := MaskBSN(""); got != "" {
		t.Fatalf("MaskBSN(\"\") = %q, want empty", got)
	}
	if got := MaskBSN("999999151"); got != Redacted {
		t.Fatalf("MaskBSN(bsn) = %q, want %q", got, Redacted)
	}
}
