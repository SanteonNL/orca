.PHONY: gen-api gen-mocks

install-tools:
	go install go.uber.org/mock/mockgen@v0.4.0
	go install github.com/vektra/mockery/v2@v2.43.2

gen-mocks:
	mockgen -destination=lib/coolfhir/client_mock.go -package=coolfhir -source=lib/coolfhir/client.go
	mockgen -destination=careplancontributor/ehr/kafka_client_mock.go -package=ehr -source=careplancontributor/ehr/kafka_client.go

load-fhir-testdata:
	curl -X POST -H "Content-Type: application/json" -d @initialize_hopital_data_bundle.json http://localhost:9090/fhir

careplancontributor.applaunch-demo:
	open "http://localhost:8080/demo-app-launch?patient=Patient/1&serviceRequest=ServiceRequest/2&practitioner=PractitionerRole/3&iss=http://localhost:9090/fhir"
