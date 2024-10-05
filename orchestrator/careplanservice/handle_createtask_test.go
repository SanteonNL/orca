package careplanservice

import (
	"errors"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func Test_basedOn(t *testing.T) {
	type args struct {
		task fhir.Task
	}
	tests := []struct {
		name    string
		args    args
		want    *string
		wantErr error
	}{
		{
			name: "basedOn references a CarePlan (OK)",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{
							Reference: to.Ptr("CarePlan/123"),
						},
					},
				},
			},
			want:    to.Ptr("CarePlan/123"),
			wantErr: nil,
		},
		{
			name: "no basedOn",
			args: args{
				task: fhir.Task{},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must have exactly one reference"),
		},
		{
			name: "basedOn contains multiple references (instead of 1)",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{},
						{},
					},
				},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must have exactly one reference"),
		},
		{
			name: "basedOn does not reference a CarePlan",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{
							Reference: to.Ptr("Patient/2"),
						},
					},
				},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must contain a relative reference to a CarePlan"),
		},
		{
			name: "basedOn is not a relative reference",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{
							Type:       to.Ptr("CarePlan"),
							Identifier: &fhir.Identifier{},
						},
					},
				},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must contain a relative reference to a CarePlan"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := basedOn(tt.args.task)
			if tt.wantErr != nil {
				require.EqualError(t, gotErr, tt.wantErr.Error())
			}
			require.Equal(t, tt.want, got)
		})
	}
}
