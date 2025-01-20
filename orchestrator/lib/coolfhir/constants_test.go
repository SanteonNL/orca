package coolfhir

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestFilterRequestHeaders(t *testing.T) {
	actual := FilterRequestHeaders(http.Header{
		"If-None-Exist":     []string{"foo"},
		"If-Match":          []string{"bar"},
		"If-None-Match":     []string{"baz"},
		"If-Modified-Since": []string{"qux"},
		"Prefer":            []string{"quux"},
		"Accept":            []string{"corge"},
		"Other":             []string{"grault"},
	})
	expected := http.Header{
		"If-None-Exist":     []string{"foo"},
		"If-Match":          []string{"bar"},
		"If-None-Match":     []string{"baz"},
		"If-Modified-Since": []string{"qux"},
		"Prefer":            []string{"quux"},
		"Accept":            []string{"corge"},
	}
	assert.Equal(t, expected, actual)
}
