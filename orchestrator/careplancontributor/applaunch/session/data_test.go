package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func TestData_Replace(t *testing.T) {
	t.Run("replaces an existing resource of the same type", func(t *testing.T) {
		data := Data{}
		data.Set("Condition/first", fhir.Condition{Id: strPtr("first")})

		data.Replace("Condition/second", fhir.Condition{Id: strPtr("second")})

		condition := Get[fhir.Condition](&data)
		require.NotNil(t, condition)
		assert.Equal(t, "second", *condition.Id)
		assert.Len(t, data.ContextResources, 1)
	})

	t.Run("leaves other resource types alone", func(t *testing.T) {
		data := Data{}
		data.Set("Patient/1", fhir.Patient{Id: strPtr("1")})
		data.Set("Condition/first", fhir.Condition{Id: strPtr("first")})

		data.Replace("Condition/second", fhir.Condition{Id: strPtr("second")})

		patient := Get[fhir.Patient](&data)
		require.NotNil(t, patient)
		assert.Equal(t, "1", *patient.Id)
		assert.Len(t, data.ContextResources, 2)
	})

	t.Run("sets when nothing of the type is present", func(t *testing.T) {
		data := Data{}
		data.Set("Patient/1", fhir.Patient{Id: strPtr("1")})

		data.Replace("Condition/only", fhir.Condition{Id: strPtr("only")})

		condition := Get[fhir.Condition](&data)
		require.NotNil(t, condition)
		assert.Equal(t, "only", *condition.Id)
	})
}

func strPtr(value string) *string {
	return &value
}
