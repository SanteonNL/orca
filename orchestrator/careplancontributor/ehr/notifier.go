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

type BundleSet struct {
	Id      string
	task    string
	Bundles []fhir.Bundle `json:"bundles"`
}

func (b *BundleSet) addBundle(bundle ...fhir.Bundle) {
	b.Bundles = append(b.Bundles, bundle...)
}

func NotifyTaskAccepted(cpsClient fhirclient.Client, kafkaClient KafkaClient, task *fhir.Task) error {
	ref := "Task/" + *task.Id
	log.Info().Msgf("NotifyTaskAccepted Task (ref=%s)", ref)
	uid, err := uuid.NewUUID()
	id := uid.URN()
	if err != nil {
		return err
	}
	bundles := BundleSet{
		Id:   id,
		task: ref,
	}

	bundle := fhir.Bundle{}

	// All resources other than tasks are not returned.
	err = cpsClient.Read("Task",
		&bundle,
		fhirclient.QueryParam("_id", *task.Id),
		//fhirclient.QueryParam("_include", "Task:focus"),
		//fhirclient.QueryParam("_include", "Task:patient"),
		//fhirclient.QueryParam("_include", "Task:requester"),
		//fhirclient.QueryParam("_include", "Task:owner"),
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

	return sendBundle(bundles, kafkaClient)
}

func findForReferences(tasks []fhir.Task) []string {
	var patientForRefs []string
	for _, task := range tasks {
		if task.For != nil {
			patientReference := task.For.Reference
			if patientReference != nil {
				log.Info().Msgf("Found patientReference %s", patientReference)
				patientForRefs = append(patientForRefs, *patientReference)
			}
		}
	}
	return patientForRefs
}

func findFocusReferences(tasks []fhir.Task) []string {
	var focusRefs []string
	for _, task := range tasks {
		if task.Focus != nil {
			focusReference := task.Focus.Reference
			if focusReference != nil {
				log.Info().Msgf("Found focusReference %s", focusReference)
				focusRefs = append(focusRefs, *focusReference)
			}
		}
	}
	return focusRefs
}
func findBasedOnReferences(tasks []fhir.Task) []string {
	var basedOnRefs []string
	for _, task := range tasks {
		if task.Focus != nil {
			basedOnReferences := task.BasedOn
			for _, reference := range basedOnReferences {
				basedOnReference := reference.Reference
				if basedOnReference != nil {
					log.Info().Msgf("Found basedOnReference %s", basedOnReference)
					basedOnRefs = append(basedOnRefs, *basedOnReference)
				}
			}
		}
	}
	return basedOnRefs
}

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

func putMapListValue(refTypeMap map[string][]string, refType string, refId string) {
	values := refTypeMap[refType]
	if values == nil {
		values = []string{refId}
	} else if !slices.Contains(values, refId) {
		values = append(values, refId)
	}
	refTypeMap[refType] = values
}

func findQuestionnaireInputs(tasks []fhir.Task) []string {
	var questionnaireRefs []string
	for _, task := range tasks {
		questionnaireRefs = append(questionnaireRefs, fetchTaskInputs(task)...)
	}
	return questionnaireRefs
}

func findQuestionnaireOutputs(tasks []fhir.Task) []string {
	var questionnaireResponseRefs []string
	for _, task := range tasks {
		questionnaireResponseRefs = append(questionnaireResponseRefs, fetchTaskOutputs(task)...)
	}
	return questionnaireResponseRefs
}

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

func isOfOutputType(output fhir.TaskOutput, typeName string) bool {
	matchesType := false
	if output.ValueReference.Type != nil {
		matchesType = *output.ValueReference.Type == typeName
	} else if output.ValueReference.Reference != nil {
		matchesType = strings.HasPrefix(*output.ValueReference.Reference, fmt.Sprintf("%s/", typeName))
	}
	return matchesType
}
func isOfInputType(input fhir.TaskInput, typeName string) bool {
	matchesType := false
	if input.ValueReference.Type != nil {
		matchesType = *input.ValueReference.Type == typeName
	} else if input.ValueReference.Reference != nil {
		matchesType = strings.HasPrefix(*input.ValueReference.Reference, fmt.Sprintf("%s/", typeName))
	}
	return matchesType
}

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
