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

	if patient == nil {
		return append(errs, errors.New("patient is required"))
	}

	log.Info().Msg("Validating Patient")
	log.Info().Msgf("Patient: %+v", *patient)

	if patient.Telecom == nil || len(patient.Telecom) == 0 {
		return append(errs, errors.New("patient telecom required"))
	}

	for _, point := range patient.Telecom {
		log.Info().Msgf("Point system: %+v", *point.System)

		if point.System != nil && *point.System == fhir.ContactPointSystemEmail {
			err := validateEmail(point.Value)
			if err != nil {
				errs = append(errs, err)
			}
		}
		if point.System != nil && *point.System == fhir.ContactPointSystemPhone {
			err := validatePhone(point.Value)
			if err != nil {
				errs = append(errs, err)
			}
		}
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
	log.Info().Msg("Validating email, email: " + *email)

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
	log.Info().Msg("Validating phone, phone: " + *phone)

	if !strings.HasPrefix(*phone, "+31") {
		return errors.New("phone number must start with +31")
	}
	return nil
}
