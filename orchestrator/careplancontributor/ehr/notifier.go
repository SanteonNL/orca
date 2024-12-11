package ehr

import (
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"slices"
	"strings"
)

type Notifier interface {
	NotifyTaskAccepted(cpsClient fhirclient.Client, task *fhir.Task) error
}

type notifier struct {
	kafkaClient KafkaClient
}

func NewNotifier(kafkaClient KafkaClient) Notifier {
	return &notifier{kafkaClient}
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
func (n *notifier) NotifyTaskAccepted(cpsClient fhirclient.Client, task *fhir.Task) error {

	ref := "Task/" + *task.Id
	log.Info().Msgf("NotifyTaskAccepted Task (ref=%s)", ref)
	uid, err := uuid.NewUUID()
	if err != nil {
		return err
	}
	id := uid.URN()
	bundles := BundleSet{
		Id:   id,
		task: ref,
	}

	bundle := fhir.Bundle{}

	// All resources other than tasks are not returned.
	err = cpsClient.Read("Task",
		&bundle,
		fhirclient.QueryParam("_id", *task.Id),
		fhirclient.QueryParam("_revinclude", "Task:part-of"),
	)
	if err != nil {
		return err
	}
	bundles.addBundle(bundle)
	var tasks []fhir.Task
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("Task"), &tasks)

	patientForRefs := findForReferences(tasks)
	log.Info().Msgf("Found %d patientForRefs", len(patientForRefs))
	result, err := fetchRefs(cpsClient, patientForRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	focusRefs := findFocusReferences(tasks)
	log.Info().Msgf("Found %d focusRefs", len(focusRefs))
	result, err = fetchRefs(cpsClient, focusRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	basedOnRefs := findBasedOnReferences(tasks)
	log.Info().Msgf("Found %d basedOnRefs", len(basedOnRefs))
	result, err = fetchRefs(cpsClient, basedOnRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	questionnaireRefs := findQuestionnaireInputs(tasks)
	log.Info().Msgf("Found %d questionnaireRefs", len(questionnaireRefs))
	result, err = fetchRefs(cpsClient, questionnaireRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	questionnaireResponseRefs := findQuestionnaireOutputs(tasks)
	result, err = fetchRefs(cpsClient, questionnaireResponseRefs)
	if err != nil {
		return err
	}
	bundles.addBundle(*result...)

	return sendBundle(bundles, n.kafkaClient)
}

// findForReferences extracts patient references from a list of tasks.
//
// Parameters:
//   - tasks: A list of FHIR tasks.
//
// Returns:
//   - []string: A list of patient references.
func findForReferences(tasks []fhir.Task) []string {
	var patientForRefs []string
	for _, task := range tasks {
		if task.For != nil {
			patientReference := task.For.Reference
			if patientReference != nil {
				log.Info().Msgf("Found patientReference %s", *patientReference)
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
func findFocusReferences(tasks []fhir.Task) []string {
	var focusRefs []string
	for _, task := range tasks {
		if task.Focus != nil {
			focusReference := task.Focus.Reference
			if focusReference != nil {
				log.Info().Msgf("Found focusReference %s", *focusReference)
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
func findBasedOnReferences(tasks []fhir.Task) []string {
	var basedOnRefs []string
	for _, task := range tasks {
		if task.Focus != nil {
			basedOnReferences := task.BasedOn
			for _, reference := range basedOnReferences {
				basedOnReference := reference.Reference
				if basedOnReference != nil {
					log.Info().Msgf("Found basedOnReference %s", *basedOnReference)
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
func fetchRefs(cpsClient fhirclient.Client, refs []string) (*[]fhir.Bundle, error) {
	var bundels []fhir.Bundle
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
		err := cpsClient.Read(refType, &bundle, fhirclient.QueryParam("_id", strings.Join(refIds, ",")))
		if err != nil {
			return nil, err
		}
		bundels = append(bundels, bundle)
	}

	return &bundels, nil
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
				matchesType := isOfOutputType(output, "QuestionnaireResponse")
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

// isOfOutputType checks if a task output is of a specific type.
//
// Parameters:
//   - output: The task output to check.
//   - typeName: The type name to check against.
//
// Returns:
//   - bool: True if the output is of the specified type, false otherwise.
func isOfOutputType(output fhir.TaskOutput, typeName string) bool {
	matchesType := false
	if output.ValueReference.Type != nil {
		matchesType = *output.ValueReference.Type == typeName
	} else if output.ValueReference.Reference != nil {
		matchesType = strings.HasPrefix(*output.ValueReference.Reference, fmt.Sprintf("%s/", typeName))
	}
	return matchesType
}

// isOfInputType checks if a task input is of a specific type.
//
// Parameters:
//   - input: The task input to check.
//   - typeName: The type name to check against.
//
// Returns:
//   - bool: True if the input is of the specified type, false otherwise.
func isOfInputType(input fhir.TaskInput, typeName string) bool {
	matchesType := false
	if input.ValueReference.Type != nil {
		matchesType = *input.ValueReference.Type == typeName
	} else if input.ValueReference.Reference != nil {
		matchesType = strings.HasPrefix(*input.ValueReference.Reference, fmt.Sprintf("%s/", typeName))
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
				matchesType := isOfInputType(input, "Questionnaire")
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
func sendBundle(set BundleSet, kafkaClient KafkaClient) error {
	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		return err
	}
	log.Info().Msgf("Sending set for task (ref=%s) to Kafka", set.task)
	err = kafkaClient.SubmitMessage(set.Id, string(jsonData))
	if err != nil {
		return err
	}

	log.Info().Msgf("Successfully send task (ref=%s) to Kafka", set.task)
	return nil
}
