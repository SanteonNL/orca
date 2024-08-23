package coolfhir

import (
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLogicalReferenceEquals(t *testing.T) {
	type args struct {
		ref   fhir.Reference
		other fhir.Reference
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Equal references",
			args: args{
				ref:   *LogicalReference("Patient", "http://example.com", "123"),
				other: *LogicalReference("Patient", "http://example.com", "123"),
			},
			want: true,
		},
		{
			name: "Different type",
			args: args{
				ref:   *LogicalReference("Patient", "http://example.com", "123"),
				other: *LogicalReference("Organization", "http://example.com", "123"),
			},
			want: false,
		},
		{
			name: "Different system",
			args: args{
				ref:   *LogicalReference("Patient", "http://example.com", "123"),
				other: *LogicalReference("Patient", "http://example.org", "123"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, LogicalReferenceEquals(tt.args.ref, tt.args.other), "LogicalReferenceEquals(%v, %v)", tt.args.ref, tt.args.other)
		})
	}
}
