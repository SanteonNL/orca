package main

import (
	"crypto/tls"
	"e2e-tests/to"
	"encoding/json"
	"fmt"
	"github.com/nuts-foundation/go-did/did"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
)

const jsonType = "application/json"

var appURL = "http://localhost:9080/hospital/orca"
var nutsAPIURL, _ = url.Parse("http://localhost:8081")
var rootFHIRBaseURL, _ = url.Parse("http://localhost:9090/fhir")
var rootFHIRClient = fhirclient.New(rootFHIRBaseURL, http.DefaultClient, nil)
var hospitalFHIRBaseURL, _ = url.Parse("http://localhost:9090/fhir/hospital")
var hospitalFHIRClient = fhirclient.New(hospitalFHIRBaseURL, http.DefaultClient, nil)
var clinicFHIRBaseURL, _ = url.Parse("http://localhost:9090/fhir/clinic")
var clinicFHIRClient = fhirclient.New(clinicFHIRBaseURL, http.DefaultClient, nil)

func main() {
	println("Creating Nuts node DIDs and Verifiable Credentials...")
	hospitalDID, err := setupDID(nutsAPIURL, "Hospital", "Amsterdam", "1234")
	if err != nil {
		panic(err)
	}
	clinicDID, err := setupDID(nutsAPIURL, "Clinic", "Utrecht", "5678")
	if err != nil {
		panic(err)
	}
	// Write an env file with hospital and clinic DIDs
	err = os.WriteFile(".env", []byte(fmt.Sprintf("HOSPITAL_DID=%s\nCLINIC_DID=%s\n", hospitalDID, clinicDID)), 0644)
	if err != nil {
		panic(err)
	}
	println("Creating HAPI FHIR tenants...")
	tenants := []string{"clinic", "hospital"}
	for i, tenantName := range tenants {
		err := createFHIRTenant(tenantName, i+1, rootFHIRClient)
		if err != nil {
			panic(fmt.Sprintf("Failed to create tenant: %s: %v", tenantName, err))
		}
	}
}

func setupDID(nutsURL *url.URL, organizationName string, organizationCity string, organizationURA string) (string, error) {
	type createDIDResponse struct {
		Documents []did.Document `json:"documents"`
	}

	// Create DID
	httpTransport := http.DefaultTransport.(*http.Transport)
	httpTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	requestBody := `{}`
	// v6.0.0-beta.8
	//httpResponse, err := http.Post(nutsURL.JoinPath("internal/vdr/v2/did").String(), jsonType, strings.NewReader(requestBody))
	// master
	httpResponse, err := http.Post(nutsURL.JoinPath("internal/vdr/v2/subject").String(), jsonType, strings.NewReader(requestBody))
	testHTTPResponse(err, httpResponse, http.StatusOK)
	var responseBody createDIDResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&responseBody); err != nil {
		return "", err
	}
	createdDID := responseBody.Documents[1].ID.String()
	println(organizationName, "DID:", createdDID)
	// Issue NutsUraCredential
	requestBody = `
	{
	  "@context": [
		"https://www.w3.org/2018/credentials/v1",
		"https://nuts.nl/credentials/2024"
	  ],
	  "type": ["VerifiableCredential", "NutsUraCredential"],
	  "issuer": "` + createdDID + `",
	  "issuanceDate": "` + time.Now().Format(time.RFC3339) + `",
	  "expirationDate": "` + time.Now().AddDate(1, 0, 0).Format(time.RFC3339) + `",
	  "credentialSubject": {
		"id": "` + createdDID + `",
		"organization": {
		  "ura": "` + organizationURA + `",
		  "name": "` + organizationName + `",
		  "city": "` + organizationCity + `"
		}
	  }
	}
`
	httpResponse, err = http.Post(nutsURL.JoinPath("internal/vcr/v2/issuer/vc").String(), jsonType, strings.NewReader(requestBody))
	testHTTPResponse(err, httpResponse, http.StatusOK)
	// Load into wallet
	httpResponse, err = http.Post(nutsURL.JoinPath("internal/vcr/v2/holder/"+createdDID+"/vc").String(), jsonType, httpResponse.Body)
	testHTTPResponse(err, httpResponse, http.StatusNoContent)

	// Register on Discovery Service
	httpResponse, err = http.Post(nutsURL.JoinPath("internal/discovery/v1/dev:HomeMonitoring2024/"+createdDID).String(), jsonType, nil)
	testHTTPResponse(err, httpResponse, http.StatusOK)
	return createdDID, nil
}

func createFHIRTenant(name string, id int, fhirClient *fhirclient.BaseClient) error {
	println("Creating tenant: " + name)
	var tenant fhir.Parameters
	tenant.Parameter = []fhir.ParametersParameter{
		{
			Name:         "id",
			ValueInteger: to.Ptr(id),
		},
		{
			Name:        "name",
			ValueString: to.Ptr(name),
		},
	}
	err := fhirClient.Create(tenant, &tenant, fhirclient.AtPath("DEFAULT/$partition-management-create-partition"))
	if err != nil && strings.Contains(err.Error(), "status=400") {
		// assume it's OK (maybe it already exists)
		return nil
	}
	return err
}

func testHTTPResponse(err error, httpResponse *http.Response, expectedStatus int) {
	if err != nil {
		panic(err)
	}
	if httpResponse.StatusCode != expectedStatus {
		responseData, _ := io.ReadAll(httpResponse.Body)
		detail := "Response data:\n----------------\n" + strings.TrimSpace(string(responseData)) + "\n----------------"
		panic(fmt.Sprintf("unexpected status code (status=%s, expected=%d, url=%s)\n%s", httpResponse.Status, expectedStatus, httpResponse.Request.URL, detail))
	}
}
