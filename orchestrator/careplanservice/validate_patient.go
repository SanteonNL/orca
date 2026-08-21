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

var (
	nonPhoneCharacters = regexp.MustCompile("[^0-9+]")
	e164               = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)
)

func validatePhone(phone *string) *validation.Error {
	if phone == nil || *phone == "" {
		return &validation.Error{Code: PhoneRequired}
	}

	normalised := normalisePhone(*phone)

	// Dutch numbers still have to be mobile — we reach the patient by SMS.
	if strings.HasPrefix(normalised, "+31") && !isDutchMobile(normalised) {
		return &validation.Error{Code: InvalidPhone}
	}

	if !e164.MatchString(normalised) {
		return &validation.Error{Code: InvalidPhone}
	}

	return nil
}

// normalisePhone strips formatting and rewrites the two national prefixes we see in EHR data to E.164.
func normalisePhone(phone string) string {
	cleaned := nonPhoneCharacters.ReplaceAllString(phone, "")

	switch {
	case strings.HasPrefix(cleaned, "00"):
		return "+" + cleaned[2:]
	case strings.HasPrefix(cleaned, "06"):
		return "+31" + cleaned[1:]
	default:
		return cleaned
	}
}

func isDutchMobile(normalised string) bool {
	return strings.HasPrefix(normalised, "+316") && len(normalised) == 12
}

const (
	EmailRequired   = "E0001"
	PhoneRequired   = "E0002"
	InvalidEmail    = "E0003"
	InvalidPhone    = "E0004"
	PatientRequired = "E9999"
)
