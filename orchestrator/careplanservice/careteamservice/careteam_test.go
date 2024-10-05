package careteamservice

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"os"
	"testing"
	"time"
)

type testCase struct {
	Key         string
	Name        string
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

func (t testCase) UpdatedTask() *fhir.Task {
	result, err := os.ReadFile("testdata/" + t.Key + "-updated-task.json")
	if err != nil {
		panic(err)
	}
	if len(result) == 0 {
		return nil
	}
	var task fhir.Task
	err = json.Unmarshal(result, &task)
	if err != nil {
		panic(err)
	}
	return &task
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

func TestUpdate(t *testing.T) {
	nowFunc = func() time.Time {
		return time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	indexData, err := os.ReadFile("testdata/index.json")
	require.NoError(t, err)
	var testCases []testCase
	err = json.Unmarshal(indexData, &testCases)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.Key+" - "+tc.Name, func(t *testing.T) {
			// Setup
			ctrl := gomock.NewController(t)
			fhirClient := mock.NewMockClient(ctrl)
			fhirClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(resource string, v *fhir.Bundle, opts ...interface{}) error {
				*v = tc.Bundle()
				return nil
			}).Times(1)
			expectedCareTeam := tc.Output()

			// Perform
			tx := coolfhir.Transaction()
			updated, err := Update(fhirClient, "1", *tc.UpdatedTask(), tx)
			require.NoError(t, err)

			// Assert
			if expectedCareTeam == nil {
				// Expected no update to CareTeam
				require.False(t, updated)
				require.Empty(t, tx.Entry)
			} else {
				// Expected update to CareTeam
				require.True(t, updated, "Expected update to CareTeam")
				var actualCareTeam fhir.CareTeam
				bundle := tx.Bundle()
				err := coolfhir.ResourceInBundle(&bundle, coolfhir.EntryHasID("10"), &actualCareTeam)
				require.NoError(t, err)
				require.Equal(t, len(expectedCareTeam.Participant), len(actualCareTeam.Participant))
				sortParticipants(expectedCareTeam.Participant)
				require.Equal(t, expectedCareTeam.Participant, actualCareTeam.Participant)
			}
		})
	}
}
