package careplancontributor

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/url"
	"testing"
)

func TestService_Integration_confirm(t *testing.T) {
	t.Skip()
	fhirBaseURL, _ := url.Parse("http://localhost:9090/fhir")
	fhirClient := fhirclient.New(fhirBaseURL, http.DefaultClient, coolfhir.Config())
	service := Service{
		CarePlanService: fhirClient,
	}

	patientRef := "Patient/1"
	serviceRequestRef := "ServiceRequest/2"
	//&practitioner=PractitionerRole/3&iss=http://localhost:9090/fhir"
	confirm, err := service.confirm(fhirClient, serviceRequestRef, patientRef)

	require.NoError(t, err)
	require.NotNil(t, confirm)

	assert.Equal(t, fhir.TaskStatusAccepted, confirm.Status)
}
