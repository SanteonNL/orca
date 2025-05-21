package lib

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"os"
	"testing"
)

func TestGenerateMain(t *testing.T) {
	bundleBytes, err := os.ReadFile("searchresult.json")
	require.NoError(t, err)
	var resource fhir.Bundle
	err = json.Unmarshal(bundleBytes, &resource)
	require.NoError(t, err)

	for _, entry := range resource.Entry {
		if entry.FullUrl == nil {
			require.Fail(t, "entry.FullUrl is nil")
		}
		println("curl -X DELETE -H @accesstoken.txt", *entry.FullUrl)
	}
}
