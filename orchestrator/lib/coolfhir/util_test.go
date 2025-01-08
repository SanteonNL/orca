package coolfhir

import (
	"errors"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
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

func Test_ValidateTaskRequiredFields(t *testing.T) {
	type args struct {
		task fhir.Task
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "invalid intent",
			args: args{
				task: fhir.Task{
					Status: fhir.TaskStatusReady,
					Intent: "invalid",
				},
			},
			wantErr: errors.New("task.Intent must be 'order'"),
		},
		{
			name: "valid intent",
			args: args{
				task: fhir.Task{
					Status: fhir.TaskStatusReady,
					Intent: "order",
				},
			},
			wantErr: nil,
		},
		{
			name: "missing task.For.Identifier.System",
			args: args{
				task: fhir.Task{
					Status: fhir.TaskStatusReady,
					Intent: "order",
					For: &fhir.Reference{
						Identifier: &fhir.Identifier{},
					},
				},
			},
			wantErr: errors.New("task.For.Identifier.System must be set"),
		},
		{
			name: "missing task.For.Identifier.Value",
			args: args{
				task: fhir.Task{
					Status: fhir.TaskStatusReady,
					Intent: "order",
					For: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
						},
					},
				},
			},
			wantErr: errors.New("task.For.Identifier.Value must be set"),
		},
		{
			name: "missing task.Requester.Identifier.System",
			args: args{
				task: fhir.Task{
					Status: fhir.TaskStatusReady,
					Intent: "order",
					For: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
							Value:  to.Ptr("value"),
						},
					},
					Requester: &fhir.Reference{
						Identifier: &fhir.Identifier{},
					},
				},
			},
			wantErr: errors.New("task.Requester.Identifier.System must be set"),
		},
		{
			name: "missing task.For.Identifier.Value",
			args: args{
				task: fhir.Task{
					Status: fhir.TaskStatusReady,
					Intent: "order",
					For: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
							Value:  to.Ptr("value"),
						},
					},
					Requester: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
						},
					},
				},
			},
			wantErr: errors.New("task.Requester.Identifier.Value must be set"),
		},
		{
			name: "missing task.Owner.Identifier.System",
			args: args{
				task: fhir.Task{
					Status: fhir.TaskStatusReady,
					Intent: "order",
					For: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
							Value:  to.Ptr("value"),
						},
					},
					Requester: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
							Value:  to.Ptr("value"),
						},
					},
					Owner: &fhir.Reference{
						Identifier: &fhir.Identifier{},
					},
				},
			},
			wantErr: errors.New("task.Owner.Identifier.System must be set"),
		},
		{
			name: "missing task.Owner.Identifier.Value",
			args: args{
				task: fhir.Task{
					Status: fhir.TaskStatusReady,
					Intent: "order",
					For: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
							Value:  to.Ptr("value"),
						},
					},
					Requester: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
							Value:  to.Ptr("value"),
						},
					},
					Owner: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
						},
					},
				},
			},
			wantErr: errors.New("task.Owner.Identifier.Value must be set"),
		},
		{
			name: "Valid - Full",
			args: args{
				task: fhir.Task{
					Status: fhir.TaskStatusReady,
					Intent: "order",
					For: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
							Value:  to.Ptr("value"),
						},
					},
					Requester: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
							Value:  to.Ptr("value"),
						},
					},
					Owner: &fhir.Reference{
						Identifier: &fhir.Identifier{
							System: to.Ptr("system"),
							Value:  to.Ptr("value"),
						},
					},
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := ValidateTaskRequiredFields(tt.args.task)
			if tt.wantErr != nil {
				require.EqualError(t, gotErr, tt.wantErr.Error())
			}
		})
	}
}

func TestIsScpSubTask(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		assert.True(t, IsScpSubTask(&fhir.Task{
			PartOf: []fhir.Reference{
				{
					Reference: to.Ptr("Task/cps-task-01"),
				},
			},
			Meta: &fhir.Meta{
				Profile: []string{SCPTaskProfile},
			},
		}))
	})
	t.Run("nil", func(t *testing.T) {
		assert.False(t, IsScpSubTask(nil))
	})
	t.Run("no partOf", func(t *testing.T) {
		assert.False(t, IsScpSubTask(&fhir.Task{
			Meta: &fhir.Meta{
				Profile: []string{SCPTaskProfile},
			},
		}))
	})
	t.Run("no meta", func(t *testing.T) {
		assert.False(t, IsScpSubTask(&fhir.Task{
			PartOf: []fhir.Reference{
				{
					Reference: to.Ptr("Task/cps-task-01"),
				},
			},
		}))
	})
	t.Run("no profile", func(t *testing.T) {
		assert.False(t, IsScpSubTask(&fhir.Task{
			PartOf: []fhir.Reference{
				{
					Reference: to.Ptr("Task/cps-task-01"),
				},
			},
			Meta: &fhir.Meta{},
		}))
	})
	t.Run("no matching profile", func(t *testing.T) {
		assert.False(t, IsScpSubTask(&fhir.Task{
			PartOf: []fhir.Reference{
				{
					Reference: to.Ptr("Task/cps-task-01"),
				},
			},
			Meta: &fhir.Meta{
				Profile: []string{"http://example.org"},
			},
		}))
	})
}

func TestValidateReference(t *testing.T) {
	type args struct {
		reference fhir.Reference
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "local reference",
			args: args{
				reference: fhir.Reference{
					Reference: to.Ptr("Patient/123"),
				},
			},
			want: true,
		},
		{
			name: "logical identifier",
			args: args{
				reference: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("123"),
					},
				},
			},
			want: true,
		},
		{
			name: "neither",
			args: args{
				reference: fhir.Reference{},
			},
			want: false,
		},
		{
			name: "both",
			args: args{
				reference: fhir.Reference{
					Reference: to.Ptr("Patient/123"),
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("123"),
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ValidateReference(tt.args.reference), "ValidateReference(%v)", tt.args.reference)
		})
	}
}

func TestFilterIdentifier(t *testing.T) {
	type args struct {
		identifiers *[]fhir.Identifier
		system      string
	}
	tests := []struct {
		name string
		args args
		want []fhir.Identifier
	}{
		{
			name: "nil identifiers",
			args: args{
				identifiers: nil,
				system:      "http://example.com",
			},
			want: []fhir.Identifier{},
		},
		{
			name: "no matching identifiers",
			args: args{
				identifiers: &[]fhir.Identifier{
					{
						System: to.Ptr("http://example.org"),
						Value:  to.Ptr("123"),
					},
				},
				system: "http://example.com",
			},
			want: []fhir.Identifier{},
		},
		{
			name: "one matching identifier",
			args: args{
				identifiers: &[]fhir.Identifier{
					{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("123"),
					},
					{
						System: to.Ptr("http://example.org"),
						Value:  to.Ptr("456"),
					},
				},
				system: "http://example.com",
			},
			want: []fhir.Identifier{
				{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
			},
		},
		{
			name: "multiple matching identifiers",
			args: args{
				identifiers: &[]fhir.Identifier{
					{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("123"),
					},
					{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("456"),
					},
				},
				system: "http://example.com",
			},
			want: []fhir.Identifier{
				{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("123"),
				},
				{
					System: to.Ptr("http://example.com"),
					Value:  to.Ptr("456"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, FilterIdentifier(tt.args.identifiers, tt.args.system), "FilterIdentifier(%v, %v)", tt.args.identifiers, tt.args.system)
		})
	}
}

func TestFilterFirstIdentifier(t *testing.T) {
	type args struct {
		identifiers *[]fhir.Identifier
		system      string
	}
	tests := []struct {
		name string
		args args
		want *fhir.Identifier
	}{
		{
			name: "nil identifiers",
			args: args{
				identifiers: nil,
				system:      "http://example.com",
			},
			want: nil,
		},
		{
			name: "no matching identifiers",
			args: args{
				identifiers: &[]fhir.Identifier{
					{
						System: to.Ptr("http://example.org"),
						Value:  to.Ptr("123"),
					},
				},
				system: "http://example.com",
			},
			want: nil,
		},
		{
			name: "one matching identifier",
			args: args{
				identifiers: &[]fhir.Identifier{
					{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("123"),
					},
					{
						System: to.Ptr("http://example.org"),
						Value:  to.Ptr("456"),
					},
				},
				system: "http://example.com",
			},
			want: &fhir.Identifier{
				System: to.Ptr("http://example.com"),
				Value:  to.Ptr("123"),
			},
		},
		{
			name: "multiple matching identifiers",
			args: args{
				identifiers: &[]fhir.Identifier{
					{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("123"),
					},
					{
						System: to.Ptr("http://example.com"),
						Value:  to.Ptr("456"),
					},
				},
				system: "http://example.com",
			},
			want: &fhir.Identifier{
				System: to.Ptr("http://example.com"),
				Value:  to.Ptr("123"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, FilterFirstIdentifier(tt.args.identifiers, tt.args.system), "FilterFirstIdentifier(%v, %v)", tt.args.identifiers, tt.args.system)
		})
	}
}

func TestParseExternalLiteralReference(t *testing.T) {
	t.Run("root path", func(t *testing.T) {
		baseURL, ref, err := ParseExternalLiteralReference("http://example.com/Patient/123", "Patient")
		require.NoError(t, err)
		assert.Equal(t, "http://example.com", baseURL.String())
		assert.Equal(t, "Patient/123", ref)
	})
	t.Run("sub path", func(t *testing.T) {
		baseURL, ref, err := ParseExternalLiteralReference("http://example.com/fhir/Patient/123", "Patient")
		require.NoError(t, err)
		assert.Equal(t, "http://example.com/fhir", baseURL.String())
		assert.Equal(t, "Patient/123", ref)
	})
	t.Run("query params", func(t *testing.T) {
		baseURL, ref, err := ParseExternalLiteralReference("http://example.com/fhir/Patient?foo=bar", "Patient")
		require.EqualError(t, err, "query parameters for external literal reference are not supported")
		assert.Nil(t, baseURL)
		assert.Empty(t, ref)
	})
}
