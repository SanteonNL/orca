//go:generate mockgen -destination=./nofifier_mock.go -package=ehr -source=notifier.go
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

type Notifier interface {
	NotifyTaskAccepted(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task) error
}

type kafkaNotifier struct {
	kafkaClient KafkaClient
}

func NewNotifier(kafkaClient KafkaClient) Notifier {
	return &kafkaNotifier{kafkaClient}
}

// BundleSet represents a set of FHIR bundles associated with a task.
type BundleSet struct {
	Id      string
	task    string
	Bundles []fhir.Bundle `json:"bundles"`
}

// addBundle adds one or more FHIR bundles to the BundleSet.
func (b *BundleSet) addBundle(bundle ...fhir.Bundle) {
	b.Bundles = append(b.Bundles, bundle...)
}

// NotifyTaskAccepted handles the notification process when a task is accepted.
// It fetches related FHIR resources and sends the data to Kafka.
//
// Parameters:
//   - task: The FHIR task that was accepted.
//
// Returns:
//   - error: An error if the notification process fails.
func (n *kafkaNotifier) NotifyTaskAccepted(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task) error {

	ref := "Task/" + *task.Id
	log.Debug().Ctx(ctx).Msgf("NotifyTaskAccepted Task (ref=%s) to Kafka", ref)
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

// findForReferences extracts patient references from a list of tasks.
//
// Parameters:
//   - tasks: A list of FHIR tasks.
//
// Returns:
//   - []string: A list of patient references.
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

// findFocusReferences extracts focus references from a list of tasks.
//
// Parameters:
//   - tasks: A list of FHIR tasks.
//
// Returns:
//   - []string: A list of focus references.
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

// findBasedOnReferences extracts based-on references from a list of tasks.
//
// Parameters:
//   - tasks: A list of FHIR tasks.
//
// Returns:
//   - []string: A list of based-on references.
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

// fetchRefs fetches FHIR bundles for a list of references.
//
// Parameters:
//   - cpsClient: The FHIR client used to fetch resources.
//   - refs: A list of references to fetch.
//
// Returns:
//   - *[]fhir.Bundle: A list of fetched FHIR bundles.
//   - error: An error if the fetch process fails.
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

// putMapListValue adds a value to a map of lists.
//
// Parameters:
//   - refTypeMap: The map to update.
//   - refType: The key for the map.
//   - refId: The value to add to the list.
func putMapListValue(refTypeMap map[string][]string, refType string, refId string) {
	values := refTypeMap[refType]
	if values == nil {
		values = []string{refId}
	} else if !slices.Contains(values, refId) {
		values = append(values, refId)
	}
	refTypeMap[refType] = values
}

// findQuestionnaireInputs extracts questionnaire input references from a list of tasks.
//
// Parameters:
//   - tasks: A list of FHIR tasks.
//
// Returns:
//   - []string: A list of questionnaire input references.
func findQuestionnaireInputs(tasks []fhir.Task) []string {
	var questionnaireRefs []string
	for _, task := range tasks {
		questionnaireRefs = append(questionnaireRefs, fetchTaskInputs(task)...)
	}
	return questionnaireRefs
}

// findQuestionnaireOutputs extracts questionnaire output references from a list of tasks.
//
// Parameters:
//   - tasks: A list of FHIR tasks.
//
// Returns:
//   - []string: A list of questionnaire output references.
func findQuestionnaireOutputs(tasks []fhir.Task) []string {
	var questionnaireResponseRefs []string
	for _, task := range tasks {
		questionnaireResponseRefs = append(questionnaireResponseRefs, fetchTaskOutputs(task)...)
	}
	return questionnaireResponseRefs
}

// fetchTaskOutputs extracts questionnaire response references from a task's outputs.
//
// Parameters:
//   - task: A FHIR task.
//
// Returns:
//   - []string: A list of questionnaire response references.
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

// isOfType checks if a given FHIR reference is of a specified type.
//
// Parameters:
//   - valueReference: The FHIR reference to check.
//   - typeName: The type name to check against.
//
// Returns:
//   - bool: True if the reference is of the specified type, false otherwise.
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

// fetchTaskInputs extracts questionnaire references from a task's inputs.
//
// Parameters:
//   - task: A FHIR task.
//
// Returns:
//   - []string: A list of questionnaire references.
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

// sendBundle sends a BundleSet to Kafka.
//
// Parameters:
//   - set: The BundleSet to send.
//   - kafkaClient: The Kafka client used to send the message.
//
// Returns:
//   - error: An error if the send process fails.
func sendBundle(ctx context.Context, set BundleSet, kafkaClient KafkaClient) error {
	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		return err
	}
	log.Debug().Ctx(ctx).Msgf("Sending set for task (ref=%s) to Kafka", set.task)
	err = kafkaClient.SubmitMessage(ctx, set.Id, string(jsonData))
	if err != nil {
		log.Warn().Ctx(ctx).Msgf("Sending set for task (ref=%s) to Kafka failed, error: %s", set.task, err.Error())
		return errors.Wrap(err, "failed to send task to Kafka")
	}

	log.Debug().Ctx(ctx).Msgf("Successfully sent task (ref=%s) to Kafka", set.task)
	return nil
}
