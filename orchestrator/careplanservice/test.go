package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"os"
)

func FHIRClientFactoryFor(fhirClient fhirclient.Client) FHIRClientFactory {
	return func(ctx context.Context) (fhirclient.Client, error) {
		return fhirClient, nil
	}
}

func mustReadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return data
}
