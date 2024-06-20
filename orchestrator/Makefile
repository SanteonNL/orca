.PHONY: gen-api gen-mocks

install-tools:
	go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.0.0
	go install go.uber.org/mock/mockgen@v0.4.0

gen-mocks:
	mockgen -destination=lib/coolfhir/client_mock.go -package=coolfhir -source=lib/coolfhir/client.go

gen-api:
	oapi-codegen -generate client,types,skip-prune -package nuts \
		-exclude-schemas VerifiableCredential,VerifiablePresentation,DID,DIDDocument \
		-o lib/nuts/generated.go https://nuts-node.readthedocs.io/en/latest/_static/common/ssi_types.yaml
	oapi-codegen -generate client,types -package iam \
		-import-mapping='../common/ssi_types.yaml:github.com/SanteonNL/orca/orchestrator/lib/nuts' \
		-o lib/nuts/iam/generated.go https://nuts-node.readthedocs.io/en/latest/_static/auth/v2.yaml
	oapi-codegen -generate client,types -package vdr \
		-import-mapping='../common/ssi_types.yaml:github.com/SanteonNL/orca/orchestrator/lib/nuts' \
		-o lib/nuts/vdr/generated.go https://nuts-node.readthedocs.io/en/latest/_static/vdr/v2.yaml

load-fhir-testdata:
	curl -X POST -H "Content-Type: application/json" -d @initialize_hopital_data_bundle.json http://localhost:9090/fhir

applaunch-demo:
	open "http://localhost:8080/demo-app-launch?patient=Patient/1&serviceRequest=ServiceRequest/2&practitioner=PractitionerRole/3&iss=http://localhost:9090/fhir"
