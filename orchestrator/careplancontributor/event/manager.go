package events

import (
	"context"
	"github.com/rs/zerolog/log"
	"sync"
)

type Manager interface {
	Subscribe(fhirResourceType string, handlerName string, handler HandleFunc) error
	Notify(ctx context.Context, fhirResourceType string, event Instance) error
}

// Instance describes an event.
type Instance struct {
	// FHIRResourceSource contains the fully qualified URL of the FHIR resource that triggered the event
	FHIRResourceSource string
	// FHIRResource contains the FHIR resource that triggered the event
	FHIRResource any
}

type Handler interface {
	Handle(ctx context.Context, event Instance) error
}

type HandleFunc func(ctx context.Context, event Instance) error

var _ Manager = &InMemoryManager{}

type inMemoryHandler struct {
	handlerName string
	handler     HandleFunc
}

type InMemoryManager struct {
	mux      sync.RWMutex
	handlers map[string][]inMemoryHandler
}

func (i *InMemoryManager) Subscribe(fhirResourceType string, handlerName string, handler HandleFunc) error {
	i.mux.Lock()
	defer i.mux.Unlock()
	if fhirResourceType == "" {
		fhirResourceType = "*"
	}
	handlers := i.handlers[fhirResourceType]
	handlers = append(handlers, inMemoryHandler{
		handlerName: handlerName,
		handler:     handler,
	})
	i.handlers[fhirResourceType] = handlers
	return nil
}

func (i *InMemoryManager) Notify(ctx context.Context, fhirResourceType string, event Instance) error {
	i.mux.RLock()
	defer i.mux.RUnlock()
	for _, handler := range append(i.handlers["*"], i.handlers[fhirResourceType]...) {
		if err := handler.handler(ctx, event); err != nil {
			log.Ctx(ctx).Err(err).Msgf("Failed to notify handler %s for event (source=%+v)", handler.handlerName, event.FHIRResourceSource)
		}
	}
	return nil
}

func NewInMemoryManager() *InMemoryManager {
	return &InMemoryManager{
		handlers: map[string][]inMemoryHandler{},
	}
}
