package policy

import (
	"encoding/json"
	"fmt"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type findSubjectFunc func(message any) (*fhir.Reference, error)

func wrapUnmarshal[T any](f func(resource T) *fhir.Reference) findSubjectFunc {
	return func(input any) (*fhir.Reference, error) {
		var (
			resource T
			err      error
		)

		data, ok := input.(json.RawMessage)
		if !ok {
			data, err = json.Marshal(input)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal resource: %w", err)
			}
		}

		if err := json.Unmarshal(data, &resource); err != nil {
			return nil, fmt.Errorf("failed to decode resource: %w", err)
		}

		return f(resource), nil
	}
}

var findSubjectFuncs = map[string]findSubjectFunc{
	"CarePlan": wrapUnmarshal(func(carePlan fhir.CarePlan) *fhir.Reference {
		return &carePlan.Subject
	}),
	"Task": wrapUnmarshal(func(task fhir.Task) *fhir.Reference {
		return task.For
	}),
	"Patient": wrapUnmarshal(func(patient fhir.Patient) *fhir.Reference {
		return &fhir.Reference{
			Id:   patient.Id,
			Type: to.Ptr("Patient"),
		}
	}),
	"ServiceRequest": wrapUnmarshal(func(serviceRequest fhir.ServiceRequest) *fhir.Reference {
		return &serviceRequest.Subject
	}),
	"Condition": wrapUnmarshal(func(condition fhir.Condition) *fhir.Reference {
		return &condition.Subject
	}),
	"Questionnaire": wrapUnmarshal(func(_ fhir.Questionnaire) *fhir.Reference {
		return nil
	}),
	"Procedure": wrapUnmarshal(func(procedure fhir.Procedure) *fhir.Reference {
		return &procedure.Subject
	}),
	"MedicationRequest": wrapUnmarshal(func(request fhir.MedicationRequest) *fhir.Reference {
		return &request.Subject
	}),
}

func findSubject(resource any, resourceType string) (*fhir.Reference, error) {
	findSubject, ok := findSubjectFuncs[resourceType]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	subject, err := findSubject(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to extract subject: %w", err)
	}

	return subject, nil
}
