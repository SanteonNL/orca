.PHONY: gen-api

install-tools:
	go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.0.0

gen-api:
	oapi-codegen -generate server,types,strict-server -exclude-schemas FHIRBundle -package api \
		-o outbound/api/generated.go outbound/api/openapi.yaml
	oapi-codegen -generate client,types,skip-prune -package nuts \
		-exclude-schemas VerifiableCredential,VerifiablePresentation,DID,DIDDocument \
		-o nuts/generated.go https://nuts-node.readthedocs.io/en/latest/_static/common/ssi_types.yaml
	oapi-codegen -generate client,types -package iam \
		-import-mapping='../common/ssi_types.yaml:github.com/SanteonNL/orca/orchestrator/nuts' \
		-o nuts/iam/generated.go https://nuts-node.readthedocs.io/en/latest/_static/auth/v2.yaml
	oapi-codegen -generate client,types -package vdr \
		-import-mapping='../common/ssi_types.yaml:github.com/SanteonNL/orca/orchestrator/nuts' \
		-o nuts/vdr/generated.go https://nuts-node.readthedocs.io/en/latest/_static/vdr/v2.yaml
