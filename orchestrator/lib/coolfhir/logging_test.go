package coolfhir

import (
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

func TestFhirUrlLoggerSanitizer(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no query parameters",
			args: args{
				in: "http://example.com",
			},
			want: "http://example.com",
		},
		{
			name: "no query parameters, question mark is retained",
			args: args{
				in: "http://example.com?",
			},
			want: "http://example.com?",
		},
		{
			name: "_include is not sanitized",
			args: args{
				in: "http://example.com?_include=foo",
			},
			want: "http://example.com?_include=foo",
		},
		{
			name: "multiple non-sanitized parameters",
			args: args{
				in: "http://example.com?_include=foo&_include=bar",
			},
			want: "http://example.com?_include=foo&_include=bar",
		},
		{
			name: "sanitized parameter",
			args: args{
				in: "http://example.com?_id=bar",
			},
			want: "http://example.com?_id=%2A%2A%2A%2A",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, _ := url.Parse(tt.args.in)

			assert.Equalf(t, tt.want, FhirUrlLoggerSanitizer(in).String(), "FhirUrlLoggerSanitizer(%v)", tt.args.in)
		})
	}
}
