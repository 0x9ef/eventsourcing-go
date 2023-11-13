# Event Sourcing in Go

The goal of this project is to implement event-sourcing pattern with minimal language requirements, and reflection usage, and to provide maximal simplicity. Also, in the future I am going to write some articles about how this library works in great detail.

## Articles

Read these articles if you wanna understand Event Sourcing solidly.
- https://jen20.dev/post/event-sourcing-in-go/
- https://dev.to/aleksk1ng/go-eventsourcing-and-cqrs-microservice-using-eventstoredb-5djo
- https://victoramartinez.com/posts/event-sourcing-in-go/
- https://martinfowler.com/eaaDev/EventSourcing.html
- https://blog.leifbattermann.de/2017/04/21/12-things-you-should-know-about-event-sourcing/

## Brief excursion

Lets separate the general Event Sourcing into 2 parts: event and aggregate cluster. 
- **Event** is a something that happened in the past. Events indicate that something within the domain has changed. They contain all the information that is needed to transform the state of the domain from one version to the next.
- **Aggregate cluster** (or event stream) is a cluster of associated objects treated as a single unit. Every aggregate has its own event stream. Therefore every event must be stored together with an identifier for its aggregate. This ID is often called AggregateId.

Each aggregate cluster has its own `Transition(Event)` function that determines in which way event should be aggregated. For example, we have `PaymentAggregate` cluster and `PaymentAggregateCreate`, `PaymentAggregateConfirmed`, `PaymentAggregateRefunded` events. 

Each event has `Reason` field that describes reason for aggregation. By this `Reason` field we can understand what exactly transition we should perform.

```go
func (pa *PaymentAggregator) Transition(evt event.Eventer) error {
	switch evt.GetReason() {
	case PaymentAggregateReasonCreated:
		return pa.onCreated(evt)
	case PaymentAggregateReasonConfirmed:
		return pa.onConfirmed(evt)
	case PaymentAggregateReasonRefunded:
		return pa.onRefunded(evt)
	}
	return errors.New("undefined event type")
}
```

### Let's implement aggregate cluster

In case of our example we will implement `PaymentAggregator` that responsibles for payment aggregations. The every aggregation structure should use `*eventsourcing.AggregateCluster` through composition (to implement `event.Aggregator` interface).

```go
type PaymentAggregator struct {
	*eventsourcing.AggregateCluster // composition
	// General.
	PaymentID              string
	PaymentStatus          string
	PaymentAmount          int
	PaymentAvailableAmount int
	PaymentRefundAmount    int
}
```

Aggregator should provide `event.Transition` function that has `func(event Eventer) error` signature. This function will be called every time when aggregate cluster does transition. Also, we have to define some event reason, each reason to each event.

```go
const (
    // Reason for paymentCreatedEvent
	PaymentAggregateReasonCreated   = "created"
    // Reason for paymentConfirmedEvent
	PaymentAggregateReasonConfirmed = "confirmed"
    // Reason for paymentRefundedEvent
	PaymentAggregateReasonRefunded  = "refunded"
)

func (pa *PaymentAggregator) Transition(evt event.Eventer) error {
	switch evt.GetReason() {
	case PaymentAggregateReasonCreated:
		return pa.onCreated(evt)
	case PaymentAggregateReasonConfirmed:
		return pa.onConfirmed(evt)
	case PaymentAggregateReasonRefunded:
		return pa.onRefunded(evt)
	}
    // In case if there is external reason that we don't know throw an error
	return errors.New("undefined event type")
}
```

### Let's define events

As far as we have 3 event reasons we should implement 3 events: "created", "confirmed", "refunded"

```go
type paymentCreatedEvent struct {
	PaymentID              string
	PaymentStatus          string
	PaymentAmount          int
	PaymentAvailableAmount int
}

func (pa *PaymentAggregator) onCreated(evt event.Eventer) error {
	var payload paymentCreatedEvent
	if err := json.Unmarshal(evt.GetPayload(), &payload); err != nil {
		return err
	}
	pa.PaymentID = payload.PaymentID
	pa.PaymentStatus = payload.PaymentStatus
	pa.PaymentAmount = payload.PaymentAmount
	pa.PaymentAvailableAmount = payload.PaymentAvailableAmount
	return nil
}

type paymentConfirmedEvent struct {
	PaymentStatus string
}

func (pa *PaymentAggregator) onConfirmed(evt event.Eventer) error {
	var payload paymentConfirmedEvent
	if err := json.Unmarshal(evt.GetPayload(), &payload); err != nil {
		return err
	}
	pa.PaymentStatus = payload.PaymentStatus
	return nil
}

type paymentRefundEvent struct {
	PaymentRefundAmount int
}

func (pa *PaymentAggregator) onRefunded(evt event.Eventer) error {
	var payload paymentRefundEvent
	if err := json.Unmarshal(evt.GetPayload(), &payload); err != nil {
		return err
	}
	pa.PaymentRefundAmount = payload.PaymentRefundAmount
	if pa.PaymentRefundAmount > pa.PaymentAmount {
		return errors.New("refund amount is greated than entire payment amount")
	}
	pa.PaymentAvailableAmount = pa.PaymentAmount - pa.PaymentRefundAmount
	return nil
}
```

### Let's compose it together and apply events

```go
func main() {
    agg := &PaymentAggregator{}
    agg.AggregateCluster = eventsourcing.New(agg, agg.Transition, eventsourcing.UUIDGenerator)

    // Define paymentCreatedEvent event to further saving
    createdEvent, err := event.New(PaymentAggregateReasonCreated, paymentCreatedEvent{
			PaymentID:              "id_0",
			PaymentStatus:          "created",
			PaymentAmount:          100,
			PaymentAvailableAmount: 100,
		})
    if err != nil {
        panic(err)
    }

    // Apply createdEvent into PaymentAggregator cluster (derivate state)
    if err := agg.Apply(createdEvent); err != nil {
        panic(err)
    }

    // Save committed events into database...
}
```

### Eventstore

At the moment only PostgreSQL supports from the box. **Note:** _table structure should be exactly as defined in `eventstore/postgresql/migrate.go`_

You can implement your own repository (for MySQL, EventStore DB, etc...) by `eventstore.Repository` interface.

### Event serialization

By default all events serializes in `JSON`. At the moment there support for: json, bson format. These formats implement `event.Serializer` interface. There is `MatchedSerializers` variable (map) that defines  `SerializerType` to serializer implementation. 
