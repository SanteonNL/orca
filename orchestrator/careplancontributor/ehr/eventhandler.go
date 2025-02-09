package ehr

import (
	"context"
	events "github.com/SanteonNL/orca/orchestrator/careplancontributor/event"
)

var _ events.Handler = EventHandler{}

type EventHandler struct {
}

func (e EventHandler) Handle(ctx context.Context, event events.Instance) error {

}
