package coolfhir

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"strings"
)

func FormatHumanName(name fhir.HumanName) string {
	if name.Text != nil {
		return *name.Text
	}
	var parts []string
	parts = append(parts, name.Prefix...)
	if name.Family != nil {
		f := *name.Family
		if len(name.Given) > 0 {
			f += ","
		}
		parts = append(parts, f)
	}
	parts = append(parts, name.Given...)
	parts = append(parts, name.Suffix...)
	return strings.Join(parts, " ")
}
