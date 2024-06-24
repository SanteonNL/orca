package coolfhir

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"net/http"
	"net/url"
)

// NewAzureClient creates a new FHIR client that communicates with an Azure FHIR API.
// It uses the Managed Identity of the Azure environment to authenticate.
func NewAzureClient(fhirBaseURL *url.URL) (fhirclient.Client, error) {
	options := azcore.ClientOptions{}
	credential, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
		ClientOptions: options,
		ID:            nil,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get credential for Azure FHIR API client: %w", err)
	}
	return newAzureClientWithCredential(fhirBaseURL, credential, options)
}

func newAzureClientWithCredential(fhirBaseURL *url.URL, credential azcore.TokenCredential, options azcore.ClientOptions) (fhirclient.Client, error) {
	azcoreClient, err := azcore.NewClient("github.com/SanteonNL/orca/orchestrator", "v0.0.0", runtime.PipelineOptions{
		PerRetry: []policy.Policy{newChallengePolicy(credential)},
		Tracing: runtime.TracingOptions{
			Namespace: "Microsoft.FHIRAPI", // TODO: Is this right?
		},
	}, &options)
	if err != nil {
		return nil, err
	}
	return fhirclient.New(fhirBaseURL, &azureHttpClient{Client: azcoreClient}), nil
}

// azureHttpClient is a wrapper around an Azure SDK HTTP client that implements the http.Client interface,
// which is then used by the fhirclient library.
type azureHttpClient struct {
	*azcore.Client
}

func (a azureHttpClient) Do(httpRequest *http.Request) (*http.Response, error) {
	azRequest, err := runtime.NewRequest(httpRequest.Context(), httpRequest.Method, httpRequest.URL.String())
	if err != nil {
		return nil, err
	}
	*azRequest.Raw() = *httpRequest
	return a.Client.Pipeline().Do(azRequest)
}

func newChallengePolicy(cred azcore.TokenCredential) policy.Policy {
	return runtime.NewBearerTokenPolicy(cred, nil, &policy.BearerTokenOptions{
		// TODO: Not sure if we ned this?
		//AuthorizationHandler: policy.AuthorizationHandler{
		//	OnRequest:   kv.authorize,
		//	OnChallenge: kv.authorizeOnChallenge,
		//},
	})
}
