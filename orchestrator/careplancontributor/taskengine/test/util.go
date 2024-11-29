package test

import (
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

import "embed"

//go:embed *.json
var fs embed.FS

func LoadQuestionnairesAndHealthcareSevices(t *testing.T, client fhirclient.Client) {
	var healthcareServiceBundle fhir.Bundle
	data, err := fs.ReadFile("healthcareservice-bundle.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &healthcareServiceBundle))
	require.NoError(t, client.Create(healthcareServiceBundle, &healthcareServiceBundle, fhirclient.AtPath("/")))

	var questionnaireBundle fhir.Bundle
	data, err = fs.ReadFile("questionnaire-bundle.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &questionnaireBundle))
	require.NoError(t, client.Create(questionnaireBundle, &questionnaireBundle, fhirclient.AtPath("/")))
}
