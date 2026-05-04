package coolfhir

import (
	"context"
	"errors"
	"testing"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/caramel/to"
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

func TestParseLocalReference(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		resourceType, resourceID, err := ParseLocalReference("")
		require.EqualError(t, err, "local reference must contain exactly one '/'")
		assert.Empty(t, resourceType)
		assert.Empty(t, resourceID)
	})
	t.Run("no slash", func(t *testing.T) {
		resourceType, resourceID, err := ParseLocalReference("Patient")
		require.EqualError(t, err, "local reference must contain exactly one '/'")
		assert.Empty(t, resourceType)
		assert.Empty(t, resourceID)
	})
	t.Run("no resource ID", func(t *testing.T) {
		resourceType, resourceID, err := ParseLocalReference("Patient/")
		require.EqualError(t, err, "local reference must contain a resource ID")
		assert.Empty(t, resourceType)
		assert.Empty(t, resourceID)
	})
	t.Run("valid", func(t *testing.T) {
		resourceType, resourceID, err := ParseLocalReference("Patient/123")
		require.NoError(t, err)
		assert.Equal(t, "Patient", resourceType)
		assert.Equal(t, "123", resourceID)
	})
}

func TestGetTaskByIdentifier(t *testing.T) {
	type args struct {
		ctx        context.Context
		fhirClient fhirclient.Client
		identifier fhir.Identifier
	}

	existingTask := fhir.Task{
		Id: to.Ptr("12345678910"),
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("unit-test-system"),
				Value:  to.Ptr("20"),
			},
		},
	}

	tests := []struct {
		name    string
		args    args
		want    *fhir.Task
		wantErr string
	}{
		{
			name: "successful search with one result",
			args: args{
				ctx: context.Background(),
				fhirClient: &test.StubFHIRClient{
					Resources: []interface{}{
						existingTask,
					},
				},
				identifier: fhir.Identifier{
					System: to.Ptr("unit-test-system"),
					Value:  to.Ptr("20"),
				},
			},
			want: &existingTask,
		},
		{
			name: "successful search with multiple results",
			args: args{
				ctx: context.Background(),
				fhirClient: &test.StubFHIRClient{
					Resources: []interface{}{
						existingTask,
						existingTask,
					},
				},
				identifier: fhir.Identifier{
					System: to.Ptr("unit-test-system"),
					Value:  to.Ptr("20"),
				},
			},
			wantErr: "found multiple existing CPS Tasks for identifier: unit-test-system|20",
		},
		{
			name: "no results found",
			args: args{
				ctx:        context.Background(),
				fhirClient: &test.StubFHIRClient{},
				identifier: fhir.Identifier{
					System: to.Ptr("unit-test-system"),
					Value:  to.Ptr("20"),
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTaskByIdentifier(tt.args.ctx, tt.args.fhirClient, tt.args.identifier)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTokenToIdentifier(t *testing.T) {
	t.Run("system and code", func(t *testing.T) {
		identifier, err := TokenToIdentifier("http://example.com|123")
		require.NoError(t, err)
		assert.Equal(t, "http://example.com", *identifier.System)
		assert.Equal(t, "123", *identifier.Value)
	})
	t.Run("system only", func(t *testing.T) {
		identifier, err := TokenToIdentifier("http://example.com|")
		require.NoError(t, err)
		assert.Equal(t, "http://example.com", *identifier.System)
		assert.Nil(t, identifier.Value)
	})
	t.Run("code only", func(t *testing.T) {
		identifier, err := TokenToIdentifier("|123")
		require.NoError(t, err)
		assert.Nil(t, identifier.System)
		assert.Equal(t, "123", *identifier.Value)
	})
	t.Run("neither", func(t *testing.T) {
		identifier, err := TokenToIdentifier("|")
		require.EqualError(t, err, "identifier search token must contain a system, or a code, or both")
		assert.Nil(t, identifier)
	})
	t.Run("invalid format", func(t *testing.T) {
		identifier, err := TokenToIdentifier("http://example.com")
		require.EqualError(t, err, "identifier search token must contain exactly one '|'")
		assert.Nil(t, identifier)
	})
}

func TestReferenceValueEquals(t *testing.T) {
	t.Run("equal references and types", func(t *testing.T) {
		ref := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
			Type:      to.Ptr("Patient"),
		}
		other := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
			Type:      to.Ptr("Patient"),
		}
		assert.True(t, ReferenceValueEquals(ref, other))
	})

	t.Run("different references", func(t *testing.T) {
		ref := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
			Type:      to.Ptr("Patient"),
		}
		other := fhir.Reference{
			Reference: to.Ptr("Patient/456"),
			Type:      to.Ptr("Patient"),
		}
		assert.False(t, ReferenceValueEquals(ref, other))
	})

	t.Run("different types", func(t *testing.T) {
		ref := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
			Type:      to.Ptr("Patient"),
		}
		other := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
			Type:      to.Ptr("Observation"),
		}
		assert.False(t, ReferenceValueEquals(ref, other))
	})

	t.Run("nil reference in ref", func(t *testing.T) {
		ref := fhir.Reference{
			Type: to.Ptr("Patient"),
		}
		other := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
			Type:      to.Ptr("Patient"),
		}
		assert.False(t, ReferenceValueEquals(ref, other))
	})

	t.Run("nil reference in other", func(t *testing.T) {
		ref := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
			Type:      to.Ptr("Patient"),
		}
		other := fhir.Reference{
			Type: to.Ptr("Patient"),
		}
		assert.False(t, ReferenceValueEquals(ref, other))
	})

	t.Run("nil type in ref", func(t *testing.T) {
		ref := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
		}
		other := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
			Type:      to.Ptr("Patient"),
		}
		assert.False(t, ReferenceValueEquals(ref, other))
	})

	t.Run("nil type in other", func(t *testing.T) {
		ref := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
			Type:      to.Ptr("Patient"),
		}
		other := fhir.Reference{
			Reference: to.Ptr("Patient/123"),
		}
		assert.False(t, ReferenceValueEquals(ref, other))
	})
}

func TestFindMatchingParticipantInCareTeam(t *testing.T) {
	id := fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("org1")}
	careTeam := &fhir.CareTeam{
		Participant: []fhir.CareTeamParticipant{
			{Member: &fhir.Reference{Identifier: &id}},
		},
	}
	t.Run("found", func(t *testing.T) {
		p := FindMatchingParticipantInCareTeam(careTeam, []fhir.Identifier{id})
		require.NotNil(t, p)
	})
	t.Run("not found", func(t *testing.T) {
		other := fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("other")}
		p := FindMatchingParticipantInCareTeam(careTeam, []fhir.Identifier{other})
		require.Nil(t, p)
	})
}

func TestParseTimestamp_UnsupportedFormat(t *testing.T) {
	_, err := ValidateCareTeamParticipantPeriod(fhir.CareTeamParticipant{
		Period: &fhir.Period{Start: to.Ptr("2024")},
	}, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported timestamp format")
}

func TestParseLocalReference_NoType(t *testing.T) {
	_, _, err := ParseLocalReference("/Patient")
	require.EqualError(t, err, "local reference must contain a resource type")
}

func TestLogicalReference(t *testing.T) {
	ref := LogicalReference("Patient", "http://example.com", "123")
	require.NotNil(t, ref)
	assert.Equal(t, "Patient", *ref.Type)
	assert.Equal(t, "http://example.com", *ref.Identifier.System)
	assert.Equal(t, "123", *ref.Identifier.Value)
}

func TestIsIdentifierTaskOwnerAndRequester(t *testing.T) {
	orgID := fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("org1")}
	otherID := fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("org2")}
	ref := func(id fhir.Identifier) *fhir.Reference {
		return &fhir.Reference{Identifier: &id}
	}

	t.Run("is owner and requester", func(t *testing.T) {
		task := &fhir.Task{Owner: ref(orgID), Requester: ref(orgID)}
		isOwner, isRequester := IsIdentifierTaskOwnerAndRequester(task, []fhir.Identifier{orgID})
		assert.True(t, isOwner)
		assert.True(t, isRequester)
	})
	t.Run("is owner only", func(t *testing.T) {
		task := &fhir.Task{Owner: ref(orgID), Requester: ref(otherID)}
		isOwner, isRequester := IsIdentifierTaskOwnerAndRequester(task, []fhir.Identifier{orgID})
		assert.True(t, isOwner)
		assert.False(t, isRequester)
	})
	t.Run("is requester only", func(t *testing.T) {
		task := &fhir.Task{Owner: ref(otherID), Requester: ref(orgID)}
		isOwner, isRequester := IsIdentifierTaskOwnerAndRequester(task, []fhir.Identifier{orgID})
		assert.False(t, isOwner)
		assert.True(t, isRequester)
	})
	t.Run("nil owner and requester", func(t *testing.T) {
		task := &fhir.Task{}
		isOwner, isRequester := IsIdentifierTaskOwnerAndRequester(task, []fhir.Identifier{orgID})
		assert.False(t, isOwner)
		assert.False(t, isRequester)
	})
}

func TestIsLocalRelativeReference(t *testing.T) {
	assert.True(t, IsLocalRelativeReference(&fhir.Reference{Reference: to.Ptr("#contained-1")}))
	assert.False(t, IsLocalRelativeReference(&fhir.Reference{Reference: to.Ptr("Patient/123")}))
	assert.False(t, IsLocalRelativeReference(nil))
	assert.False(t, IsLocalRelativeReference(&fhir.Reference{}))
}

func TestIsLogicalReference(t *testing.T) {
	assert.True(t, IsLogicalReference(&fhir.Reference{
		Type:       to.Ptr("Patient"),
		Identifier: &fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("123")},
	}))
	assert.False(t, IsLogicalReference(nil))
	assert.False(t, IsLogicalReference(&fhir.Reference{}))
}

func TestIsLogicalIdentifier(t *testing.T) {
	assert.True(t, IsLogicalIdentifier(&fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("123")}))
	assert.False(t, IsLogicalIdentifier(nil))
	assert.False(t, IsLogicalIdentifier(&fhir.Identifier{System: to.Ptr("http://example.com")}))
}

func TestLogicalReferenceEquals(t *testing.T) {
	ref := fhir.Reference{Identifier: &fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("123")}}
	same := fhir.Reference{Identifier: &fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("123")}}
	diff := fhir.Reference{Identifier: &fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("456")}}

	assert.True(t, LogicalReferenceEquals(ref, same))
	assert.False(t, LogicalReferenceEquals(ref, diff))
	assert.False(t, LogicalReferenceEquals(fhir.Reference{}, ref))
}

func TestHasIdentifier(t *testing.T) {
	id := fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("123")}
	other := fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("456")}

	assert.True(t, HasIdentifier(id, id, other))
	assert.False(t, HasIdentifier(id, other))
}

func TestToString(t *testing.T) {
	id := fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("123")}
	assert.Equal(t, "http://example.com|123", ToString(id))
	assert.Equal(t, "http://example.com|123", ToString(&id))
	assert.Contains(t, ToString(42), "42")
}

func TestIdentifierToToken(t *testing.T) {
	id := fhir.Identifier{System: to.Ptr("http://example.com"), Value: to.Ptr("123")}
	assert.Equal(t, "http://example.com|123", IdentifierToToken(id))
}

func TestHttpMethodToVerb(t *testing.T) {
	assert.Equal(t, fhir.HTTPVerbPUT, HttpMethodToVerb("PUT"))
	assert.Equal(t, fhir.HTTPVerbPOST, HttpMethodToVerb("POST"))
	assert.Equal(t, fhir.HTTPVerbDELETE, HttpMethodToVerb("DELETE"))
	assert.Equal(t, fhir.HTTPVerbPATCH, HttpMethodToVerb("PATCH"))
	assert.Equal(t, fhir.HTTPVerbGET, HttpMethodToVerb("GET"))
	assert.Equal(t, fhir.HTTPVerbHEAD, HttpMethodToVerb("HEAD"))
	assert.Equal(t, fhir.HTTPVerb(0), HttpMethodToVerb("UNKNOWN"))
}

func TestIsScpTask(t *testing.T) {
	t.Run("true when meta has SCP profile", func(t *testing.T) {
		task := &fhir.Task{Meta: &fhir.Meta{Profile: []string{SCPTaskProfile}}}
		assert.True(t, IsScpTask(task))
	})
	t.Run("false when meta is nil", func(t *testing.T) {
		assert.False(t, IsScpTask(&fhir.Task{}))
	})
	t.Run("false when profile does not match", func(t *testing.T) {
		task := &fhir.Task{Meta: &fhir.Meta{Profile: []string{"other-profile"}}}
		assert.False(t, IsScpTask(task))
	})
}

func TestConceptContainsCoding(t *testing.T) {
	coding := fhir.Coding{System: to.Ptr("http://example.com"), Code: to.Ptr("code1")}
	concept := fhir.CodeableConcept{Coding: []fhir.Coding{coding}}

	assert.True(t, ConceptContainsCoding(coding, concept))
	assert.False(t, ConceptContainsCoding(fhir.Coding{System: to.Ptr("http://example.com"), Code: to.Ptr("other")}, concept))
}

func TestContainsCoding(t *testing.T) {
	coding := fhir.Coding{System: to.Ptr("http://example.com"), Code: to.Ptr("code1")}

	assert.True(t, ContainsCoding(coding, coding))
	assert.False(t, ContainsCoding(coding, fhir.Coding{System: to.Ptr("http://example.com"), Code: to.Ptr("other")}))
	assert.False(t, ContainsCoding(fhir.Coding{}, coding))
}

func TestOrganizationIdentifiers(t *testing.T) {
	org1 := fhir.Organization{Identifier: []fhir.Identifier{{System: to.Ptr("http://example.com"), Value: to.Ptr("org1")}}}
	org2 := fhir.Organization{Identifier: []fhir.Identifier{{System: to.Ptr("http://example.com"), Value: to.Ptr("org2")}}}

	result := OrganizationIdentifiers([]fhir.Organization{org1, org2})
	require.Len(t, result, 2)
	assert.Equal(t, "org1", *result[0].Value)
	assert.Equal(t, "org2", *result[1].Value)
}
