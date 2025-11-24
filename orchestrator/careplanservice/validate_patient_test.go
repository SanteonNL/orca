package careplanservice

import (
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
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
			expectedErr: []string{PatientRequired},
		},
		{
			name:        "rejects patient with no telecom",
			patient:     &fhir.Patient{},
			expectedErr: []string{EmailRequired, PhoneRequired},
		},
		{
			name: "rejects patient with empty telecom",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{},
			},
			expectedErr: []string{EmailRequired, PhoneRequired},
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
			expectedErr: []string{EmailRequired, PhoneRequired},
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
			expectedErr: []string{InvalidEmail},
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
			expectedErr: []string{EmailRequired},
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
			expectedErr: []string{EmailRequired},
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
			expectedErr: []string{InvalidPhone},
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
			expectedErr: []string{InvalidPhone},
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
			expectedErr: []string{InvalidPhone},
		},
		{
			name: "accepts patient with dutch phone containing spaces",
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
			expectedErr: []string{InvalidPhone},
		},
		{
			name: "accepts patient with valid dutch phone starting with 06",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("0612345678"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "rejects patient with nil phone value",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  nil,
					},
				},
			},
			expectedErr: []string{PhoneRequired},
		},
		{
			name: "rejects patient with empty phone value",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr(""),
					},
				},
			},
			expectedErr: []string{PhoneRequired},
		},
		{
			name: "rejects patient with only email",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
				},
			},
			expectedErr: []string{PhoneRequired},
		},
		{
			name: "rejects patient with only phone",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31612345678"),
					},
				},
			},
			expectedErr: []string{EmailRequired},
		},
		{
			name: "accepts patient with at lease one valid phone number",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+36oops"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31612345678"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+31112345678"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "accepts Belgian mobile number (international format)",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+32478123456"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "accepts German mobile number (international format)",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+491711234567"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "accepts German mobile with 015 prefix",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+491521234567"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "accepts German mobile with 016 prefix",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+491601234567"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "rejects invalid Belgian number (wrong prefix)",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("0278123456"),
					},
				},
			},
			expectedErr: []string{InvalidPhone},
		},
		{
			name: "rejects invalid German number (wrong prefix)",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("0211234567"),
					},
				},
			},
			expectedErr: []string{InvalidPhone},
		},
		{
			name: "accepts mix of NL, BE, and DE numbers",
			patient: &fhir.Patient{
				Telecom: []fhir.ContactPoint{
					{
						System: &emailSystem,
						Value:  to.Ptr("test@example.com"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("0612345678"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+32478123456"),
					},
					{
						System: &phoneSystem,
						Value:  to.Ptr("+491711234567"),
					},
				},
			},
			expectedErr: nil,
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
