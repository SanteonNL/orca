package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestPatientValidator_Validate(t *testing.T) {
	emailSystem := fhir.ContactPointSystemEmail
	phoneSystem := fhir.ContactPointSystemPhone
	faxSystem := fhir.ContactPointSystemFax

	tests := []struct {
		name        string
		patient     *fhir.Patient
		expectedErr []string
	}{
		{
			name: "accepts patient with valid email and phone",
			patient: &fhir.Patient{
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
			},
			expectedErr: nil,
		},
		{
			name:        "rejects nil patient",
			patient:     nil,
			expectedErr: []string{"patient is required"},
		},
		{
			name:        "rejects patient with no telecom",
			patient:     &fhir.Patient{},
			expectedErr: []string{"patient telecom required"},
		},
		{
			name: "rejects patient with empty telecom",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{},
			},
			expectedErr: []string{"patient telecom required"},
		},
		{
			name: "rejects patient with nil telecom",
			patient: &fhir.Patient{
				Telecom: nil,
			},
			expectedErr: []string{"patient telecom required"},
		},
		{
			name: "rejects patient with only other contact point systems",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &faxSystem,
						Value:  to.Ptr("123456789"),
					},
				},
			},
			expectedErr: []string{"patient must have both email and phone"},
		},
		{
			name: "rejects patient with invalid email but valid phone",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("invalid-email"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31612345678"),
					},
				},
			},
			expectedErr: []string{"email is invalid"},
		},
		{
			name: "rejects patient with nil email value",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  nil,
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31612345678"),
					},
				},
			},
			expectedErr: []string{"email is required"},
		},
		{
			name: "rejects patient with empty email value",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr(""),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31612345678"),
					},
				},
			},
			expectedErr: []string{"email is required"},
		},
		{
			name: "accepts patient with valid dutch mobile phone starting with +316",
			patient: &fhir.Patient{
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
			},
			expectedErr: nil,
		},
		{
			name: "accepts patient with valid dutch phone with minimum digits",
			patient: &fhir.Patient{
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
			},
			expectedErr: nil,
		},
		{
			name: "rejects patient with dutch phone number too short",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+3161234567"),
					},
				},
			},
			expectedErr: []string{"patient phone number should be a dutch mobile number"},
		},
		{
			name: "rejects patient with dutch phone number too long",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31612345678901"),
					},
				},
			},
			expectedErr: []string{"patient phone number should be a dutch mobile number"},
		},
		{
			name: "rejects patient with dutch phone containing non-numeric characters",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+3161234567a"),
					},
				},
			},
			expectedErr: []string{"patient phone number should be a dutch mobile number"},
		},
		{
			name: "Accepts patient with dutch phone containing spaces",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31 6 12345678"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "accepts patient with dutch phone containing dashes",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31-6-12345678"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "rejects patient with phone starting with +31 but invalid area code",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31112345678"),
					},
				},
			},
			expectedErr: []string{"patient phone number should be a dutch mobile number"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &PatientValidator{}
			errs := validator.Validate(tt.patient)

			if tt.expectedErr == nil {
				assert.Nil(t, errs)
			} else {
				assert.NotNil(t, errs)
				assert.Len(t, errs, len(tt.expectedErr))
				for i, expected := range tt.expectedErr {
					assert.Contains(t, errs[i].Error(), expected)
				}
			}
		})
	}
}
