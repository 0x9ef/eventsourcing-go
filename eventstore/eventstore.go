package eventstore

import (
	"context"

	"github.com/0x9ef/eventsourcing-go/event"
)

type Repository interface {
	Get(ctx context.Context, aggregateID, aggregateType string, version event.Version) (event.Eventer, error)
	List(ctx context.Context, aggregateID, aggregateType string, filter *ListFilter) ([]event.Eventer, error)
	Save(ctx context.Context, events []event.Eventer) error
}

type ListFilter struct {
	AfterVersion  event.Version
	BeforeVersion event.Version
	Limit         int
}
