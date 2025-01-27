//go:generate mockgen -destination=./notifier_mock.go -package=ehr -source=notifier.go
package ehr

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// Notifier is an interface for sending notifications regarding task acceptance within a FHIR-based system.
type Notifier interface {
	NotifyTaskAccepted(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task) error
}

// kafkaNotifier is a type that uses a ServiceBusClient to send messages to a Kafka service.
type kafkaNotifier struct {
	kafkaClient ServiceBusClient
}

// NewNotifier creates and returns a Notifier implementation using the provided ServiceBusClient for message handling.
func NewNotifier(kafkaClient ServiceBusClient) Notifier {
	return &kafkaNotifier{kafkaClient}
}

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

// NotifyTaskAccepted sends notification data comprehensively related to a specific FHIR Task to a Kafka service bus.
func (n *kafkaNotifier) NotifyTaskAccepted(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task) error {

	ref := "Task/" + *task.Id
	log.Debug().Ctx(ctx).Msgf("NotifyTaskAccepted Task (ref=%s) to ServiceBus", ref)
	id := uuid.NewString()
	bundles := BundleSet{
		Id:   id,
		task: ref,
	}

	bundle := fhir.Bundle{}

	// All resources other than tasks are not returned.
	values := url.Values{}
	values.Set("_id", *task.Id)
	values.Set("_revinclude", "Task:part-of")
	err := cpsClient.SearchWithContext(ctx, "Task", values, &bundle)
	if err != nil {
		return err
	}
	bundles.addBundle(bundle)
	var tasks []fhir.Task
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("Task"), &tasks)
	if err != nil {
		return err
	}

	patientForRefs := findForReferences(ctx, tasks)
	log.Debug().Ctx(ctx).Msgf("Found %d patientForRefs", len(patientForRefs))
	result, err := fetchRefs(ctx, cpsClient, patientForRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	focusRefs := findFocusReferences(ctx, tasks)
	log.Debug().Ctx(ctx).Msgf("Found %d focusRefs", len(focusRefs))
	result, err = fetchRefs(ctx, cpsClient, focusRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	basedOnRefs := findBasedOnReferences(ctx, tasks)
	log.Debug().Ctx(ctx).Msgf("Found %d basedOnRefs", len(basedOnRefs))
	result, err = fetchRefs(ctx, cpsClient, basedOnRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	questionnaireRefs := findQuestionnaireInputs(tasks)
	log.Debug().Ctx(ctx).Msgf("Found %d questionnaireRefs", len(questionnaireRefs))
	result, err = fetchRefs(ctx, cpsClient, questionnaireRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	questionnaireResponseRefs := findQuestionnaireOutputs(tasks)
	result, err = fetchRefs(ctx, cpsClient, questionnaireResponseRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	return sendBundle(ctx, bundles, n.kafkaClient)
}

// findForReferences retrieves a list of patient references from the "For" field in the provided list of fhir.Task objects.
func findForReferences(ctx context.Context, tasks []fhir.Task) []string {
	var patientForRefs []string
	for _, task := range tasks {
		if task.For != nil {
			patientReference := task.For.Reference
			if patientReference != nil {
				log.Debug().Ctx(ctx).Msgf("Found patientReference %s", *patientReference)
				patientForRefs = append(patientForRefs, *patientReference)
			}
		}
	}
	return patientForRefs
}

// findFocusReferences extracts and returns a list of focus references from the provided FHIR tasks, if available.
func findFocusReferences(ctx context.Context, tasks []fhir.Task) []string {
	var focusRefs []string
	for _, task := range tasks {
		if task.Focus != nil {
			focusReference := task.Focus.Reference
			if focusReference != nil {
				log.Debug().Ctx(ctx).Msgf("Found focusReference %s", *focusReference)
				focusRefs = append(focusRefs, *focusReference)
			}
		}
	}
	return focusRefs
}

// findBasedOnReferences retrieves a list of references from the "BasedOn" field of the given tasks, filtering out nil references.
func findBasedOnReferences(ctx context.Context, tasks []fhir.Task) []string {
	var basedOnRefs []string
	for _, task := range tasks {
		if task.Focus != nil {
			basedOnReferences := task.BasedOn
			for _, reference := range basedOnReferences {
				basedOnReference := reference.Reference
				if basedOnReference != nil {
					log.Debug().Ctx(ctx).Msgf("Found basedOnReference %s", *basedOnReference)
					basedOnRefs = append(basedOnRefs, *basedOnReference)
				}
			}
		}
	}
	return basedOnRefs
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
		var bundle fhir.Bundle
		values := url.Values{}
		values.Set("_id", strings.Join(refIds, ","))
		if refType == "CarePlan" {
			values.Set("_include", "CarePlan:care-team")
		}
		err := cpsClient.SearchWithContext(ctx, refType, values, &bundle)
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, bundle)
	}

	return &bundles, nil
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

// findQuestionnaireInputs extracts and returns references to "Questionnaire" resources from the input tasks.
func findQuestionnaireInputs(tasks []fhir.Task) []string {
	var questionnaireRefs []string
	for _, task := range tasks {
		questionnaireRefs = append(questionnaireRefs, fetchTaskInputs(task)...)
	}
	return questionnaireRefs
}

// findQuestionnaireOutputs extracts references to "QuestionnaireResponse" outputs from a list of FHIR tasks.
func findQuestionnaireOutputs(tasks []fhir.Task) []string {
	var questionnaireResponseRefs []string
	for _, task := range tasks {
		questionnaireResponseRefs = append(questionnaireResponseRefs, fetchTaskOutputs(task)...)
	}
	return questionnaireResponseRefs
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

// sendBundle sends a serialized BundleSet to a Service Bus using the provided ServiceBusClient.
// It logs the process and errors during submission while wrapping and returning them.
// Returns an error if serialization or message submission fails.
func sendBundle(ctx context.Context, set BundleSet, kafkaClient ServiceBusClient) error {
	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		return err
	}
	log.Debug().Ctx(ctx).Msgf("Sending set for task (ref=%s) to ServiceBus", set.task)
	err = kafkaClient.SubmitMessage(ctx, set.Id, string(jsonData))
	if err != nil {
		log.Warn().Ctx(ctx).Msgf("Sending set for task (ref=%s) to ServiceBus failed, error: %s", set.task, err.Error())
		return errors.Wrap(err, "failed to send task to ServiceBus")
	}

	log.Debug().Ctx(ctx).Msgf("Successfully sent task (ref=%s) to ServiceBus", set.task)
	return nil
}
