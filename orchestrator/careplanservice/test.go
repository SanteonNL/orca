package careplanservice

import (
	"net/http"
	"os"
)

type MockPolicyMiddleware struct{}

func (m MockPolicyMiddleware) Use(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next(w, r)
	})
}

func NewMockPolicyMiddleware() MockPolicyMiddleware {
	return MockPolicyMiddleware{}
}

func mustReadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return data
}
