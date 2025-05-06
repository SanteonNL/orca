package coolfhir

import (
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestFormatHumanName(t *testing.T) {
	type args struct {
		name fhir.HumanName
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "text",
			args: args{
				name: fhir.HumanName{
					Text:   to.Ptr("John Doe"),
					Family: to.Ptr("Doe"),
				},
			},
			want: "John Doe",
		},
		{
			name: "Test with only family name",
			args: args{
				name: fhir.HumanName{
					Family: to.Ptr("Doe"),
				},
			},
			want: "Doe",
		},
		{
			name: "Test with all fields",
			args: args{
				name: fhir.HumanName{
					Text:   nil,
					Family: to.Ptr("Doe"),
					Given:  []string{"John", "James"},
					Prefix: []string{"Mr."},
					Suffix: []string{"Jr."},
				},
			},
			want: "Mr. Doe, John James Jr.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, FormatHumanName(tt.args.name), "FormatHumanName(%v)", tt.args.name)
		})
	}
}
