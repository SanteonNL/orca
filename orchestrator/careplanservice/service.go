package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"net/http"
)

func New(didResolver addressing.DIDResolver) *Service {
	return &Service{
		didResolver: didResolver,
	}
}

type Service struct {
	didResolver addressing.DIDResolver
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/careplans", s.example)
}

func (s Service) example(response http.ResponseWriter, request *http.Request) {
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("Hello, careplan!"))
}
