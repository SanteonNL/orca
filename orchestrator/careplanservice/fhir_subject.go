package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type findSubjectFunc func(message json.RawMessage) (*fhir.Reference, error)

func wrapUnmarshal[T any](f func(resource T) *fhir.Reference) findSubjectFunc {
	return func(message json.RawMessage) (*fhir.Reference, error) {
		var resource T

		if err := json.Unmarshal(message, &resource); err != nil {
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
}

func findSubject(ctx context.Context, fhirClient fhirclient.Client, resourceType, id string) (*fhir.Reference, error) {
	path := fmt.Sprintf("%s/%s", resourceType, id)

	findFunc, ok := findSubjectFuncs[resourceType]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	var message json.RawMessage

	if err := fhirClient.ReadWithContext(ctx, path, &message); err != nil {
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	subject, err := findFunc(message)
	if err != nil {
		return nil, fmt.Errorf("failed to extract subject: %w", err)
	}

	return subject, nil
}
