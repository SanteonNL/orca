package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestPatientValidator_Validate(t *testing.T) {
	validator := &PatientValidator{}

	t.Run("accepts patient with valid email and phone", func(t *testing.T) {
		emailSystem := fhir.ContactPointSystemEmail
		phoneSystem := fhir.ContactPointSystemPhone
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &emailSystem,
					Value:  to.Ptr("test@example.com"),
				},
				{
					System: &phoneSystem,
					Value:  to.Ptr("+31612345678"),
				},
			},
		}

		errs := validator.Validate(patient)

		assert.Nil(t, errs)
	})

	t.Run("rejects nil patient", func(t *testing.T) {
		errs := validator.Validate(nil)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "patient is required")
	})

	t.Run("rejects patient with no telecom", func(t *testing.T) {
		patient := &fhir.Patient{}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "patient telecom required")
	})

	t.Run("rejects patient with empty telecom", func(t *testing.T) {
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{},
		}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "patient telecom required")
	})

	t.Run("rejects patient with nil telecom", func(t *testing.T) {
		patient := &fhir.Patient{
			Telecom: nil,
		}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "patient telecom required")
	})

	t.Run("accepts patient with other contact point systems", func(t *testing.T) {
		faxSystem := fhir.ContactPointSystemFax
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &faxSystem,
					Value:  to.Ptr("123456789"),
				},
			},
		}

		errs := validator.Validate(patient)

		assert.Nil(t, errs)
	})

	t.Run("rejects patient with invalid email", func(t *testing.T) {
		emailSystem := fhir.ContactPointSystemEmail
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &emailSystem,
					Value:  to.Ptr("invalid-email"),
				},
			},
		}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "email is invalid")
	})

	t.Run("rejects patient with nil email value", func(t *testing.T) {
		emailSystem := fhir.ContactPointSystemEmail
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &emailSystem,
					Value:  nil,
				},
			},
		}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "email is required")
	})

	t.Run("rejects patient with empty email value", func(t *testing.T) {
		emailSystem := fhir.ContactPointSystemEmail
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &emailSystem,
					Value:  to.Ptr(""),
				},
			},
		}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "email is required")
	})

	t.Run("rejects patient with phone not starting with +31", func(t *testing.T) {
		phoneSystem := fhir.ContactPointSystemPhone
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &phoneSystem,
					Value:  to.Ptr("+32123456789"),
				},
			},
		}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "phone number must start with +31")
	})

	t.Run("rejects patient with nil phone value", func(t *testing.T) {
		phoneSystem := fhir.ContactPointSystemPhone
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &phoneSystem,
					Value:  nil,
				},
			},
		}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "phone number is required")
	})

	t.Run("rejects patient with empty phone value", func(t *testing.T) {
		phoneSystem := fhir.ContactPointSystemPhone
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &phoneSystem,
					Value:  to.Ptr(""),
				},
			},
		}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "phone number is required")
	})

	t.Run("accepts patient with valid phone starting with +31", func(t *testing.T) {
		phoneSystem := fhir.ContactPointSystemPhone
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &phoneSystem,
					Value:  to.Ptr("+31987654321"),
				},
			},
		}

		errs := validator.Validate(patient)

		assert.Nil(t, errs)
	})

	t.Run("validates multiple contact points", func(t *testing.T) {
		emailSystem := fhir.ContactPointSystemEmail
		phoneSystem := fhir.ContactPointSystemPhone
		faxSystem := fhir.ContactPointSystemFax
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &emailSystem,
					Value:  to.Ptr("test@example.com"),
				},
				{
					System: &phoneSystem,
					Value:  to.Ptr("+31612345678"),
				},
				{
					System: &faxSystem,
					Value:  to.Ptr("123456789"),
				},
			},
		}

		errs := validator.Validate(patient)

		assert.Nil(t, errs)
	})

	t.Run("stops validation at first invalid contact point", func(t *testing.T) {
		emailSystem := fhir.ContactPointSystemEmail
		phoneSystem := fhir.ContactPointSystemPhone
		patient := &fhir.Patient{
			Telecom: []fhir.ContactPoint{
				{
					System: &emailSystem,
					Value:  to.Ptr("invalid-email"),
				},
				{
					System: &phoneSystem,
					Value:  to.Ptr("+32123456789"),
				},
			},
		}

		errs := validator.Validate(patient)

		assert.NotNil(t, errs)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Error(), "email is invalid")
	})
}
