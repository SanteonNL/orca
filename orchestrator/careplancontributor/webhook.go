package careplancontributor

import (
	"context"
	events "github.com/SanteonNL/orca/orchestrator/careplancontributor/event"
)

var _ events.Handler = WebhookEventHandler{}

type WebhookEventHandler struct {
	URL string
}

func (w WebhookEventHandler) Handle(ctx context.Context, event events.Instance) error {
	panic("implement me")
}
