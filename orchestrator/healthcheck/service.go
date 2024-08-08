package healthcheck

import (
	"encoding/json"
	"net/http"
)

func New() *Service {
	return &Service{}
}

type Service struct{}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealthCheck)
}

func (s Service) handleHealthCheck(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(map[string]string{"status": "up"})
}
