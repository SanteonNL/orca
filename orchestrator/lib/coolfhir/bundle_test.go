package coolfhir

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTransactionBuilder(t *testing.T) {
	tx := Transaction().
		Create(fhir.Task{
			Id: to.Ptr("task1"),
		}).
		Create(fhir.Task{
			Id: to.Ptr("task2"),
		}).
		Bundle()

	require.Equal(t, fhir.BundleTypeTransaction, tx.Type)
	require.Len(t, tx.Entry, 2)

	var task1 map[string]interface{}
	require.NoError(t, json.Unmarshal(tx.Entry[0].Resource, &task1))
	require.Equal(t, "task1", task1["id"])
	var task2 map[string]interface{}
	require.NoError(t, json.Unmarshal(tx.Entry[1].Resource, &task2))
	require.Equal(t, "task2", task2["id"])
}
