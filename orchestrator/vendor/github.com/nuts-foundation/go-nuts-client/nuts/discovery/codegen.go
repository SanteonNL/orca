//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -o generated.go --config=config.yaml https://nuts-node.readthedocs.io/en/latest/_static/discovery/v1.yaml

package discovery

type N200Status string
