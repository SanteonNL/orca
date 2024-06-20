package coolfhir

import fhirclient "github.com/SanteonNL/go-fhir-client"

type ClientCreator func(properties map[string]string) fhirclient.Client

var ClientFactories = map[string]ClientCreator{}
