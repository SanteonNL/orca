package ehr

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// BundleSet represents a collection of FHIR bundles associated with a specific task, identified by an ID.
type BundleSet struct {
	Id      string
	task    string
	Bundles []fhir.Bundle `json:"bundles"`
}

// addBundle adds one or more FHIR bundles to the BundleSet's Bundles slice.
func (b *BundleSet) addBundle(bundle ...fhir.Bundle) {
	b.Bundles = append(b.Bundles, bundle...)
}

func TaskNotificationBundleSet(ctx context.Context, cpsClient fhirclient.Client, taskId string) (*BundleSet, error) {
	ref := "Task/" + taskId
	log.Ctx(ctx).Debug().Msgf("NotifyTaskAccepted Task (ref=%s) to ServiceBus", ref)
	id := uuid.NewString()
	bundles := BundleSet{
		Id:   id,
		task: ref,
	}

	taskBundle, tasks, err := fetchTasks(ctx, cpsClient, taskId)
	if err != nil {
		return nil, err
	}

	carePlanBundle, carePlan, err := fetchCarePlan(ctx, cpsClient, tasks)
	if err != nil {
		return nil, err
	}

	patientBundle, err := fetchPatient(ctx, cpsClient, carePlan)
	if err != nil {
		return nil, err
	}

	serviceRequestBundle, err := fetchServiceRequest(ctx, cpsClient, tasks)
	if err != nil {
		return nil, err
	}

	questionnaireBundles, err := fetchQuestionnaires(ctx, cpsClient, tasks)
	if err != nil {
		return nil, err
	}

	questionnaireResponseBundles, err := fetchQuestionnaireResponses(ctx, cpsClient, tasks)
	if err != nil {
		return nil, err
	}

	// Bundle order is important for the ServiceBus message
	bundles.addBundle(*taskBundle)
	bundles.addBundle(*patientBundle)
	bundles.addBundle(*serviceRequestBundle)
	bundles.addBundle(*carePlanBundle)
	bundles.addBundle(*questionnaireBundles...)
	bundles.addBundle(*questionnaireResponseBundles...)

	return &bundles, nil
}

func fetchTasks(ctx context.Context, cpsClient fhirclient.Client, taskId string) (*fhir.Bundle, []fhir.Task, error) {
	var taskBundle fhir.Bundle

	values := url.Values{}
	values.Set("_id", taskId)
	values.Set("_revinclude", "Task:part-of")
	err := cpsClient.SearchWithContext(ctx, "Task", values, &taskBundle)
	if err != nil {
		return nil, nil, err
	}
	if len(taskBundle.Entry) < 2 {
		return nil, nil, fmt.Errorf("expected at least 2 Tasks (1 primary, 1 subtask), got %d", len(taskBundle.Entry))
	}
	var tasks []fhir.Task
	err = coolfhir.ResourcesInBundle(&taskBundle, coolfhir.EntryIsOfType("Task"), &tasks)
	if err != nil {
		return nil, nil, err
	}

	return &taskBundle, tasks, nil
}

func fetchCarePlan(ctx context.Context, cpsClient fhirclient.Client, tasks []fhir.Task) (*fhir.Bundle, *fhir.CarePlan, error) {
	var carePlanRefs []string
	for _, task := range tasks {
		if task.Focus != nil {
			basedOnReferences := task.BasedOn
			for _, reference := range basedOnReferences {
				basedOnReference := reference.Reference
				if basedOnReference != nil && !slices.Contains(carePlanRefs, *basedOnReference) {
					log.Ctx(ctx).Debug().Msgf("Found carePlanRef %s", *basedOnReference)
					carePlanRefs = append(carePlanRefs, *basedOnReference)
				}
			}
		}
	}
	if len(carePlanRefs) != 1 {
		return nil, nil, fmt.Errorf("expected exactly 1 CarePlan, got %d", len(carePlanRefs))
	}
	log.Ctx(ctx).Debug().Msgf("Found %d carePlanRefs", len(carePlanRefs))
	carePlanBundle, err := fetchRef(ctx, cpsClient, carePlanRefs[0])
	if err != nil {
		return nil, nil, err
	}

	var carePlan fhir.CarePlan
	err = coolfhir.ResourceInBundle(carePlanBundle, coolfhir.EntryIsOfType("CarePlan"), &carePlan)
	if err != nil {
		return nil, nil, err
	}
	if carePlan.Subject.Reference == nil {
		return nil, nil, fmt.Errorf("could not find patient reference in CarePlan")
	}

	return carePlanBundle, &carePlan, nil
}

func fetchPatient(ctx context.Context, cpsClient fhirclient.Client, carePlan *fhir.CarePlan) (*fhir.Bundle, error) {
	patientRef := *carePlan.Subject.Reference
	patientBundle, err := fetchRef(ctx, cpsClient, patientRef)
	if err != nil {
		return nil, err
	}
	return patientBundle, nil
}

func fetchServiceRequest(ctx context.Context, cpsClient fhirclient.Client, tasks []fhir.Task) (*fhir.Bundle, error) {
	var serviceRequestRefs []string
	for _, task := range tasks {
		if task.Focus != nil {
			focusReference := task.Focus.Reference
			if focusReference != nil && !slices.Contains(serviceRequestRefs, *focusReference) {
				log.Ctx(ctx).Debug().Msgf("Found serviceRequestRef %s", *focusReference)
				serviceRequestRefs = append(serviceRequestRefs, *focusReference)
			}
		}
	}
	if len(serviceRequestRefs) != 1 {
		return nil, fmt.Errorf("expected exactly 1 ServiceRequest, got %d", len(serviceRequestRefs))
	}
	log.Ctx(ctx).Debug().Msgf("Found %d serviceRequestRefs", len(serviceRequestRefs))
	serviceRequestBundle, err := fetchRef(ctx, cpsClient, serviceRequestRefs[0])
	if err != nil {
		return nil, err
	}
	return serviceRequestBundle, nil
}

func fetchQuestionnaires(ctx context.Context, cpsClient fhirclient.Client, tasks []fhir.Task) (*[]fhir.Bundle, error) {
	var questionnaireRefs []string
	for _, task := range tasks {
		questionnaireRefs = append(questionnaireRefs, fetchTaskInputs(task)...)
	}
	log.Ctx(ctx).Debug().Msgf("Found %d questionnaireRefs", len(questionnaireRefs))
	questionnaireBundle, err := fetchRefs(ctx, cpsClient, questionnaireRefs)
	if err != nil {
		return nil, err
	}
	return questionnaireBundle, nil
}

func fetchQuestionnaireResponses(ctx context.Context, cpsClient fhirclient.Client, tasks []fhir.Task) (*[]fhir.Bundle, error) {
	var questionnaireResponseRefs []string
	for _, task := range tasks {
		questionnaireResponseRefs = append(questionnaireResponseRefs, fetchTaskOutputs(task)...)
	}
	log.Ctx(ctx).Debug().Msgf("Found %d questionnaireResponseRefs", len(questionnaireResponseRefs))
	questionnaireResponseBundle, err := fetchRefs(ctx, cpsClient, questionnaireResponseRefs)
	if err != nil {
		return nil, err
	}
	return questionnaireResponseBundle, nil
}

// fetchRefs retrieves FHIR Bundles for the provided resource references using the FHIR client and returns the resulting bundles.
// It organizes references by resource type, executes FHIR searches for each type, and handles errors during the search process.
// The function supports including CareTeam resources for CarePlan references when constructing the query parameters.
// Returns a pointer to a slice of FHIR Bundles and an error, if any occurred during FHIR client interactions.
func fetchRefs(ctx context.Context, cpsClient fhirclient.Client, refs []string) (*[]fhir.Bundle, error) {
	var bundles []fhir.Bundle
	var refTypeMap = make(map[string][]string)
	for _, ref := range refs {
		splits := strings.Split(ref, "/")
		if len(splits) < 1 {
			continue
		}
		refType := splits[0]
		refId := splits[1]
		putMapListValue(refTypeMap, refType, refId)
	}

	for refType, refIds := range refTypeMap {
		expectedResourceCount := len(refIds)

		var bundle fhir.Bundle
		values := url.Values{}
		values.Set("_id", strings.Join(refIds, ","))
		err := cpsClient.SearchWithContext(ctx, refType, values, &bundle)
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, bundle)

		if expectedResourceCount != len(bundle.Entry) {
			return nil, fmt.Errorf("failed to fetch all references of type %s, expected %d bundle entries, got %d", refType, expectedResourceCount, len(bundle.Entry))
		}
	}

	return &bundles, nil
}

// fetchRef performs the same operations as fetchRefs, but only for a single resource
func fetchRef(ctx context.Context, cpsClient fhirclient.Client, ref string) (*fhir.Bundle, error) {
	var refTypeMap = make(map[string][]string)
	splits := strings.Split(ref, "/")
	if len(splits) < 1 {
		return nil, fmt.Errorf("invalid reference: %s", ref)
	}
	refType := splits[0]
	refId := splits[1]
	putMapListValue(refTypeMap, refType, refId)

	expectedResourceCount := 1

	var bundle fhir.Bundle
	values := url.Values{}
	values.Set("_id", refId)
	err := cpsClient.SearchWithContext(ctx, refType, values, &bundle)
	if err != nil {
		return nil, err
	}

	if expectedResourceCount != len(bundle.Entry) {
		return nil, fmt.Errorf("failed to fetch all references of type %s, expected %d bundle entries, got %d", refType, expectedResourceCount, len(bundle.Entry))
	}

	return &bundle, nil
}

// putMapListValue adds a reference ID to a map of string slices if not already present, grouped by reference type.
func putMapListValue(refTypeMap map[string][]string, refType string, refId string) {
	values := refTypeMap[refType]
	if values == nil {
		values = []string{refId}
	} else if !slices.Contains(values, refId) {
		values = append(values, refId)
	}
	refTypeMap[refType] = values
}

// fetchTaskOutputs retrieves unique references to `QuestionnaireResponse` outputs from a given FHIR Task.
func fetchTaskOutputs(task fhir.Task) []string {
	var questionnaireResponseRefs []string
	if task.Output != nil {
		for _, output := range task.Output {
			if output.ValueReference != nil &&
				output.ValueReference.Reference != nil {
				matchesType := isOfType(output.ValueReference, "QuestionnaireResponse")
				if matchesType {
					reference := *output.ValueReference.Reference
					if !slices.Contains(questionnaireResponseRefs, reference) {
						questionnaireResponseRefs = append(questionnaireResponseRefs, reference)
					}
				}
			}
		}
	}
	return questionnaireResponseRefs
}

// isOfType checks if a given FHIR reference matches the specified type name based on its Type or Reference field.
func isOfType(valueReference *fhir.Reference, typeName string) bool {
	matchesType := false
	if valueReference.Type != nil {
		matchesType = *valueReference.Type == typeName
	} else if valueReference.Reference != nil {
		if strings.HasPrefix(*valueReference.Reference, "https://") {
			compile, err := regexp.Compile(fmt.Sprintf("^https:/.*/%s/(.+)$", typeName))
			if err != nil {
				log.Error().Msgf("Failed to compile regex: %s", err.Error())
			} else {
				matchesType = compile.MatchString(*valueReference.Reference)
			}
		} else {
			matchesType = strings.HasPrefix(*valueReference.Reference, fmt.Sprintf("%s/", typeName))
		}
	}
	return matchesType
}

// fetchTaskInputs extracts and returns a list of questionnaire references from the inputs of the given FHIR Task.
// It ensures references are unique and belong to the type "Questionnaire".
func fetchTaskInputs(task fhir.Task) []string {
	var questionnaireRefs []string
	if task.Input != nil {
		for _, input := range task.Input {
			if input.ValueReference != nil &&
				input.ValueReference.Reference != nil {
				matchesType := isOfType(input.ValueReference, "Questionnaire")
				if matchesType {
					reference := *input.ValueReference.Reference
					if !slices.Contains(questionnaireRefs, reference) {
						questionnaireRefs = append(questionnaireRefs, reference)
					}
				}
			}
		}
	}
	return questionnaireRefs
}
