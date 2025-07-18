package coolfhir

import (
	"encoding/json"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func marshalContained(contained []any) json.RawMessage {
	data, err := json.Marshal(contained)
	if err != nil {
		panic(err)
	}

	return data
}

func TestCareTeamFromCarePlan(t *testing.T) {
	tests := map[string]struct {
		carePlan       fhir.CarePlan
		expectedResult fhir.CareTeam
		expectedErr    string
	}{
		"careteam is resolved properly": {
			carePlan: fhir.CarePlan{
				CareTeam: []fhir.Reference{
					{
						Reference: to.Ptr("#careteam"),
						Type:      to.Ptr("CareTeam"),
					},
				},
				Contained: marshalContained([]any{
					fhir.CareTeam{Id: to.Ptr("careteam")},
				}),
			},
			expectedResult: fhir.CareTeam{
				Id: to.Ptr("careteam"),
			},
			expectedErr: "",
		},
		"more than one careteam": {
			carePlan: fhir.CarePlan{
				CareTeam: []fhir.Reference{
					{
						Reference: to.Ptr("#careteam"),
					},
					{
						Reference: to.Ptr("#careteam2"),
					},
				},
			},
			expectedErr: "CarePlan must have exactly one CareTeam",
		},
		"invalid reference": {
			carePlan: fhir.CarePlan{
				CareTeam: []fhir.Reference{
					{
						Reference: to.Ptr("invalid"),
					},
				},
			},
			expectedErr: "invalid CareTeam reference",
		},
		"careteam not found": {
			carePlan: fhir.CarePlan{
				CareTeam: []fhir.Reference{
					{
						Reference: to.Ptr("#careteam"),
					},
				},
				Contained: marshalContained([]any{
					fhir.CareTeam{Id: to.Ptr("careteam2")},
				}),
			},
			expectedErr: "failed to resolve CareTeam",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := CareTeamFromCarePlan(&tt.carePlan)

			if tt.expectedErr == "" {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tt.expectedResult, *result)
			} else {
				require.Nil(t, result)
				require.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}

func TestUpdateContainedResource(t *testing.T) {
	t.Run("updating works", func(t *testing.T) {
		id := fhir.Reference{
			Reference: to.Ptr("#careteam"),
			Type:      to.Ptr("CareTeam"),
		}
		carePlan := fhir.CarePlan{
			CareTeam: []fhir.Reference{id},
			Contained: marshalContained([]any{
				fhir.CareTeam{Id: to.Ptr("careteam")},
			}),
		}

		expected := fhir.CareTeam{
			Id: to.Ptr("careteam"),
			Participant: []fhir.CareTeamParticipant{
				{
					Id: to.Ptr("participant-1"),
				},
			},
		}

		contained, err := UpdateContainedResource(carePlan.Contained, &id, expected)

		require.NoError(t, err)
		require.Equal(t, string(marshalContained([]any{expected})), string(contained))
	})
}
