package coolfhir

import (
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

// Workflow that performs the HL7 FHIR Workflow Option H: https://hl7.org/fhir/R4/workflow-management.html#optionh
// TL;DR:
//  1. Placer POSTs a Task to the Broker (CarePlanService)
//  2. Broker POSTs a Task to the Filler (CarePlanContributor)
//  3. Filler accepts the Task by setting its status to accepted
//  4. Placer checks the status of the Task
type Workflow struct {
	CarePlanService Client
}

func (w Workflow) Invoke(task any) (*fhir.Task, error) {
	var result fhir.Task
	err := w.CarePlanService.Create("Task", task, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (w Workflow) TaskStatus(taskId string) (fhir.TaskStatus, error) {
	var task fhir.Task
	err := w.CarePlanService.Read("Task/"+taskId, &task)
	if err != nil {
		return -1, err
	}
	return task.Status, nil
}
