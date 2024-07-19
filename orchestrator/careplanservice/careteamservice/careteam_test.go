package careteamservice

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

type testCase struct {
	Key         string
	Description string
}

func (t testCase) Bundle() fhir.Bundle {
	result, err := os.ReadFile("testdata/" + t.Key + "-input.json")
	if err != nil {
		panic(err)
	}
	var bundle fhir.Bundle
	err = json.Unmarshal(result, &bundle)
	if err != nil {
		panic(err)
	}
	return bundle
}

func (t testCase) Output() *fhir.CareTeam {
	result, err := os.ReadFile("testdata/" + t.Key + "-output.json")
	if err != nil {
		panic(err)
	}
	if len(result) == 0 {
		return nil
	}
	var careTeam fhir.CareTeam
	err = json.Unmarshal(result, &careTeam)
	if err != nil {
		panic(err)
	}
	return &careTeam
}

func TestUpdateCareTeam(t *testing.T) {
	indexData, err := os.ReadFile("testdata/index.json")
	require.NoError(t, err)
	var testCases []testCase
	err = json.Unmarshal(indexData, &testCases)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.Key+" - "+tc.Description, func(t *testing.T) {
			result, err := updateCareTeam(tc.Bundle(), "1")
			require.NoError(t, err)

			expectedCareTeam := tc.Output()
			if expectedCareTeam == nil {
				// No expected changes in CareTeam
				require.Nil(t, result)
			} else {
				// CareTeam changed
				require.NotNil(t, result)
				require.Equal(t, expectedCareTeam.Participant, result.Participant)
				// Check participants are correct
			outer:
				for _, expectedParticipant := range expectedCareTeam.Participant {
					for _, participant := range result.Participant {
						err := coolfhir.ValidateLogicalReference(participant.OnBehalfOf, coolfhir.TypeOrganization, coolfhir.URANamingSystem)
						require.NoError(t, err)
						if *participant.OnBehalfOf.Identifier.Value == to.Value(expectedParticipant.OnBehalfOf.Identifier.Value) {
							// OK, found
							continue outer
						}
					}
					assert.Failf(t, "Participant not found in CareTeam, URA: %s", to.Value(expectedParticipant.OnBehalfOf.Identifier.Value))
				}
			}
		})
	}
}
