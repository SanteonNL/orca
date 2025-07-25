package careplanservice

import (
	"errors"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/mail"
	"strings"
)

type PatientValidator struct {
}

func (v *PatientValidator) Validate(patient *fhir.Patient) []error {
	var errs []error
	hasEmail, hasPhone := false, false

	if patient == nil {
		errs = append(errs, errors.New("patient is required"))
		return errs
	}

	log.Info().Msgf("Validating Patient: %+v", *patient)

	if patient.Telecom == nil || len(patient.Telecom) == 0 {
		errs = append(errs, errors.New("patient telecom required"))
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
				if err := validatePhone(point.Value); err != nil {
					errs = append(errs, err)
				}
				hasPhone = true
			default:
				continue
			}
		}
	}

	if !hasEmail && !hasPhone {
		errs = append(errs, errors.New("patient must have both email and phone"))
	} else if !hasEmail {
		errs = append(errs, errors.New("patient must have email"))
	} else if !hasPhone {
		errs = append(errs, errors.New("patient must have phone"))
	}

	if len(errs) > 0 {
		log.Error().Msgf("Validation errors: %v", errs)
		return errs
	}
	return nil
}

func validateEmail(email *string) error {

	if email == nil || *email == "" {
		return errors.New("email is required")
	}

	_, err := mail.ParseAddress(*email)
	if err != nil {
		return errors.New("email is invalid")
	}
	return nil
}

func validatePhone(phone *string) error {
	if phone == nil || *phone == "" {
		return errors.New("phone number is required")
	}

	return isDutchPhoneNumber(phone)
}

func isDutchPhoneNumber(phone *string) error {
	normalised := strings.NewReplacer("-", "", " ", "", "(", "", ")", "").Replace(strings.TrimSpace(*phone))

	normalised = strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r == '+' {
			return r
		}
		return -1
	}, normalised)

	if len(normalised) == 10 && strings.HasPrefix(normalised, "06") {
		return nil
	}

	if len(normalised) == 12 && strings.HasPrefix(normalised, "+316") {
		return nil
	}
	return errors.New("patient phone number should be a dutch mobile number")

}
