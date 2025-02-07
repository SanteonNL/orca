package webhook

import (
	"context"
	events "github.com/SanteonNL/orca/orchestrator/careplancontributor/event"
)

var _ events.Handler = EventHandler{}

type EventHandler struct {
	URL string
}

func (w EventHandler) Handle(ctx context.Context, event events.Instance) error {
	panic("implement me")
}
