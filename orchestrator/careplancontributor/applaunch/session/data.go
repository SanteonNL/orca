package session

import (
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"strings"
)

type FHIRResource struct {
	Path     string
	Resource *any
}

type Data struct {
	// FHIRLauncher is the name of the FHIRLauncher (FHIR client factory) that should be used to create a FHIR client
	// to interact with the EHR's FHIR API.
	FHIRLauncher string
	// TaskIdentifier is the FHIR logical identifier of the Task that is used to launch the app.
	TaskIdentifier *string
	// ContextResources contains FHIR resources that define the context of the app launch, e.g. the Patient and Practitioner resource.
	ContextResources []FHIRResource
	//Other              any
	LauncherProperties map[string]string
}

func (d *Data) Set(path string, resource any) {
	res := FHIRResource{
		Path: path,
	}
	if resource != nil {
		res.Resource = &resource
	}
	d.ContextResources = append(d.ContextResources, res)
}

func (d *Data) Get(resourceType string) *FHIRResource {
	for _, resource := range d.ContextResources {
		if strings.HasPrefix(resource.Path, resourceType+"/") {
			return &resource
		}
	}
	return nil
}

func Get[T any](data *Data) *T {
	var zero T
	resource := data.Get(coolfhir.ResourceType(zero))
	if resource == nil {
		return nil
	}
	result, ok := (*resource.Resource).(T)
	if !ok {
		return nil
	}
	return &result
}
