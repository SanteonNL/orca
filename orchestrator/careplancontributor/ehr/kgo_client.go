//go:generate mockgen -destination=./kgo_client_mock.go -package=ehr -source=kgo_client.go
package ehr

import (
	"context"
	"github.com/twmb/franz-go/pkg/kgo"
)

type KgoClient interface {
	Ping(ctx context.Context) error
	ProduceSync(ctx context.Context, rs ...*kgo.Record) kgo.ProduceResults
	Flush(ctx context.Context) error
}

type KgoClientImpl struct {
	client *kgo.Client
}

func (k KgoClientImpl) Ping(ctx context.Context) error {
	return k.Ping(ctx)
}

func (k KgoClientImpl) ProduceSync(ctx context.Context, rs ...*kgo.Record) kgo.ProduceResults {
	return k.client.ProduceSync(ctx, rs...)
}

func (k KgoClientImpl) Flush(ctx context.Context) error {
	return k.Flush(ctx)
}

func NewKgoClient(client *kgo.Client) KgoClient {
	return KgoClientImpl{client: client}
}
func NewKgoClientWithOpts(opts []kgo.Opt) (KgoClient, error) {
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return KgoClientImpl{client: client}, nil
}
