package careplanservice

import (
	"net/http"
	"os"

	"github.com/SanteonNL/orca/orchestrator/lib/policy"
)

type MockPolicyMiddleware struct{}

func (m MockPolicyMiddleware) WrapWithPolicyCheck(extractContext policy.ContextExtractor, next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next(w, r)
	})
}

func NewMockPolicyMiddleware() PolicyMiddleware {
	return MockPolicyMiddleware{}
}

func mustReadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return data
}
