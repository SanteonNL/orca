package coolfhir

import (
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestValidateLogicalReference(t *testing.T) {
	type args struct {
		reference      *fhir.Reference
		expectedType   string
		expectedSystem string
	}
	tests := []struct {
		name    string
		args    args
		wantErr string
	}{
		{
			name: "valid reference",
			args: args{
				reference:      LogicalReference("Patient", "http://example.com", "123"),
				expectedSystem: "http://example.com",
				expectedType:   "Patient",
			},
		},
		{
			name: "missing type",
			args: args{
				reference: &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("123"),
					},
				},
				expectedSystem: "http://example.com",
				expectedType:   "Patient",
			},
			wantErr: "reference.Type must be Patient",
		},
		{
			name: "wrong type",
			args: args{
				reference: &fhir.Reference{
					Type: to.Ptr("Observation"),
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("123"),
					},
				},
				expectedSystem: "http://example.com",
				expectedType:   "Patient",
			},
			wantErr: "reference.Type must be Patient",
		},
		{
			name: "missing identifier",
			args: args{
				reference: &fhir.Reference{
					Type: to.Ptr("Patient"),
				},
				expectedSystem: "http://example.com",
				expectedType:   "Patient",
			},
			wantErr: "reference must contain a logical identifier with a System and Value",
		},
		{
			name: "missing system",
			args: args{
				reference: &fhir.Reference{
					Type: to.Ptr("Patient"),
					Identifier: &fhir.Identifier{
						Value: to.Ptr("123"),
					},
				},
				expectedSystem: "http://example.com",
				expectedType:   "Patient",
			},
			wantErr: "reference must contain a logical identifier with a System and Value",
		},
		{
			name: "missing value",
			args: args{
				reference: &fhir.Reference{
					Type: to.Ptr("Patient"),
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://example.com"),
					},
				},
				expectedSystem: "http://example.com",
				expectedType:   "Patient",
			},
			wantErr: "reference must contain a logical identifier with a System and Value",
		},
		{
			name: "wrong system",
			args: args{
				reference: &fhir.Reference{
					Type: to.Ptr("Patient"),
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://example.org"),
						Value:  to.Ptr("123"),
					},
				},
				expectedSystem: "http://example.com",
				expectedType:   "Patient",
			},
			wantErr: "reference.Identifier.System must be http://example.com",
		},
		{
			name: "nil reference",
			args: args{
				reference:      nil,
				expectedSystem: "http://example.com",
				expectedType:   "Patient",
			},
			wantErr: "reference cannot be nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLogicalReference(tt.args.reference, tt.args.expectedType, tt.args.expectedSystem)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIdentifierEquals(t *testing.T) {
	type args struct {
		one   *fhir.Identifier
		other *fhir.Identifier
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				one: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
				other: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
			},
			want: true,
		},
		{
			name: "different system",
			args: args{
				one: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
				other: &fhir.Identifier{
					System: to.Ptr("http://example.org"),
					Value:  to.Ptr("123"),
				},
			},
			want: false,
		},
		{
			name: "different value",
			args: args{
				one: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
				other: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("456"),
				},
			},
			want: false,
		},
		{
			name: "nil one",
			args: args{
				one:   nil,
				other: &fhir.Identifier{},
			},
			want: false,
		},
		{
			name: "nil other",
			args: args{
				one:   &fhir.Identifier{},
				other: nil,
			},
			want: false,
		},
		{
			name: "nil both",
			args: args{
				one:   nil,
				other: nil,
			},
		},
		{
			name: "nil system one",
			args: args{
				one: &fhir.Identifier{
					Value: to.Ptr("123"),
				},
				other: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
			},
			want: false,
		},
		{
			name: "nil system other",
			args: args{
				one: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
			},
			want: false,
		},
		{
			name: "nil value one",
			args: args{
				one: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
				},
				other: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
			},
			want: false,
		},
		{
			name: "nil value other",
			args: args{
				one: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
				other: &fhir.Identifier{
					System: to.Ptr("http://example.com"),
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, IdentifierEquals(tt.args.one, tt.args.other), "IdentifierEquals(%v, %v)", tt.args.one, tt.args.other)
		})
	}
}

func TestValidateCareTeamParticipantPeriod(t *testing.T) {
	type args struct {
		participant fhir.CareTeamParticipant
	}
	tests := []struct {
		name       string
		args       args
		wantResult bool
		wantErr    string
	}{
		{
			name: "No period - Fail",
			args: args{
				participant: fhir.CareTeamParticipant{},
			},
			wantResult: false,
			wantErr:    "CareTeamParticipant has nil period",
		},
		{
			name: "No start date - Fail",
			args: args{
				participant: fhir.CareTeamParticipant{
					Period: &fhir.Period{},
				},
			},
			wantResult: false,
			wantErr:    "CareTeamParticipant has nil start date",
		},
		{
			name: "Start date in future - Fail",
			args: args{
				participant: fhir.CareTeamParticipant{
					Period: &fhir.Period{
						Start: to.Ptr(time.Now().Add(time.Minute * 5).Format(time.RFC3339)),
					},
				},
			},
			wantResult: false,
			wantErr:    "",
		},
		{
			name: "Start date in past - Valid",
			args: args{
				participant: fhir.CareTeamParticipant{
					Period: &fhir.Period{
						Start: to.Ptr(time.Now().Add(time.Minute * -5).Format(time.RFC3339)),
					},
				},
			},
			wantResult: true,
			wantErr:    "",
		},
		{
			name: "End date in past - Fail",
			args: args{
				participant: fhir.CareTeamParticipant{
					Period: &fhir.Period{
						Start: to.Ptr(time.Now().Add(time.Minute * -5).Format(time.RFC3339)),
						End:   to.Ptr(time.Now().Add(time.Minute * -3).Format(time.RFC3339)),
					},
				},
			},
			wantResult: false,
			wantErr:    "",
		},
		{
			name: "End date in future - Valid",
			args: args{
				participant: fhir.CareTeamParticipant{
					Period: &fhir.Period{
						Start: to.Ptr(time.Now().Add(time.Minute * -5).Format(time.RFC3339)),
						End:   to.Ptr(time.Now().Add(time.Minute * 5).Format(time.RFC3339)),
					},
				},
			},
			wantResult: true,
			wantErr:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateCareTeamParticipantPeriod(tt.args.participant, time.Now())
			assert.Equal(t, tt.wantResult, result)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
