package careplanservice

import (
	"log/slog"
	"net/mail"
	"regexp"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/validation"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type PatientValidator struct {
}

func (v *PatientValidator) Validate(patient *fhir.Patient) []*validation.Error {
	var errs []*validation.Error
	hasEmail, hasPhone := false, false
	hasValidPhoneNumber := false

	if patient == nil {
		errs = append(errs, &validation.Error{
			Code: PatientRequired,
		})
		return errs
	}

	for _, point := range patient.Telecom {
		if point.System != nil {
			switch *point.System {
			case fhir.ContactPointSystemEmail:
				if err := validateEmail(point.Value); err != nil {
					errs = append(errs, err)
				}
				hasEmail = true
			case fhir.ContactPointSystemPhone:
				if point.Value != nil && *point.Value != "" {
					hasPhone = true
					if err := validatePhone(point.Value); err == nil {
						hasValidPhoneNumber = true
					}
				}
			default:
				continue
			}
		}
	}

	if !hasEmail {
		errs = append(errs, &validation.Error{Code: EmailRequired})
	}
	if !hasPhone {
		errs = append(errs, &validation.Error{Code: PhoneRequired})
	}
	if hasPhone && !hasValidPhoneNumber {
		errs = append(errs, &validation.Error{Code: InvalidPhone})
	}

	if len(errs) > 0 {
		slog.Debug("Validation errors", slog.Any("errors", errs))
		return errs
	}
	return nil
}

func validateEmail(email *string) *validation.Error {

	if email == nil || *email == "" {
		return &validation.Error{Code: EmailRequired}
	}

	_, err := mail.ParseAddress(*email)
	if err != nil {
		return &validation.Error{Code: InvalidEmail}
	}
	return nil
}

func validatePhone(phone *string) *validation.Error {
	if phone == nil || *phone == "" {
		return &validation.Error{Code: PhoneRequired}
	}

	normalised := regexp.MustCompile("[^0-9+]").ReplaceAllString(*phone, "")

	// Dutch mobile: 06xxxxxxxx (10 digits) or +316xxxxxxxx (12 digits)
	if (len(normalised) == 10 && strings.HasPrefix(normalised, "06")) ||
		(len(normalised) == 12 && strings.HasPrefix(normalised, "+316")) {
		return nil
	}

	return &validation.Error{Code: InvalidPhone}
}

const (
	EmailRequired   = "E0001"
	PhoneRequired   = "E0002"
	InvalidEmail    = "E0003"
	InvalidPhone    = "E0004"
	PatientRequired = "E9999"
)
