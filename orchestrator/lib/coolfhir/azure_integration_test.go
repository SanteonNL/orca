package coolfhir

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
	"os"
	"testing"
)

func TestAzure_Integration(t *testing.T) {
	t.Skip("Needs to be configured to run")
	os.Setenv("AZURE_TENANT_ID", "")
	os.Setenv("AZURE_CLIENT_ID", "")
	os.Setenv("AZURE_CLIENT_SECRET", "")
	fhirBaseURL, _ := url.Parse("https://(...).fhir.azurehealthcareapis.com/")

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	require.NoError(t, err)
	fhirClient := NewAzureFHIRClient(fhirBaseURL, credential, DefaultAzureScope(fhirBaseURL))

	require.NoError(t, err)

	var patient = fhir.Patient{}
	err = fhirClient.Create(patient, &patient)
	require.NoError(t, err)

	var actual fhir.Patient
	err = fhirClient.Read("Patient/"+*patient.Id, &actual)

	require.NoError(t, err)
	require.NotEmpty(t, actual.Id)
}
