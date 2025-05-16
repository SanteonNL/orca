package must

import (
	"encoding/json"
	"net/url"
)

func ParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic("invalid URL: " + err.Error())
	}
	return u
}

func MarshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("failed to marshal JSON: " + err.Error())
	}
	return b
}

func UnmarshalJSON[T any](b []byte) T {
	var result T
	err := json.Unmarshal(b, &result)
	if err != nil {
		panic("failed to unmarshal JSON: " + err.Error())
	}
	return result
}
