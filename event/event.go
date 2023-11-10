package event

import (
	"encoding/json"
	"time"
)

// Eventer is a main interface with all basic getters/setters
// that responsibles for event manipulation.
type Eventer interface {
	GetAggregateId() string
	SetAggregateId(id string)
	GetAggregateType() string
	SetAggregateType(typ string)
	GetReason() string
	SetReason(reason string)
	GetVersion() Version
	SetVersion(version Version)
	GetTimestamp() Timestamp
	SetTimestamp(tstamp Timestamp)
	GetPayload() Payload
	SetPayload(payload Payload)
}

// Version represents event version.
// Default version is DirtyVersion.
type Version int

const (
	DirtyVersion Version = -1
	EmptyVersion Version = 0
	NextVersion  Version = 1
)

// Timestamp represents an event timestamp when event was created.
type Timestamp time.Time

// Payload represents an event payload (sequence of bytes).
type Payload []byte

type Event struct {
	aggregateId   string
	aggregateType string
	reason        string
	version       Version
	tstamp        Timestamp
	payload       Payload
}

var _ (Eventer) = &Event{}

func New(reason string, payload interface{}) (*Event, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Event{
		reason:  reason,
		payload: b,
	}, nil
}

func MustNew(reason string, payload interface{}) *Event {
	event, err := New(reason, payload)
	if err != nil {
		panic(err)
	}
	return event
}

func (evt *Event) GetAggregateId() string {
	return evt.aggregateId
}

func (evt *Event) SetAggregateId(aggId string) {
	evt.aggregateId = aggId
}

func (evt *Event) GetAggregateType() string {
	return evt.aggregateType
}

func (evt *Event) SetAggregateType(aggType string) {
	evt.aggregateType = aggType
}

func (evt *Event) GetReason() string {
	return evt.reason
}

func (evt *Event) SetReason(reason string) {
	evt.reason = reason
}

func (evt *Event) GetVersion() Version {
	return evt.version
}

func (evt *Event) SetVersion(version Version) {
	evt.version = version
}

func (evt *Event) GetTimestamp() Timestamp {
	return evt.tstamp
}

func (evt *Event) SetTimestamp(tstamp Timestamp) {
	evt.tstamp = tstamp
}

func (evt *Event) GetPayload() Payload {
	return evt.payload
}

func (evt *Event) SetPayload(payload Payload) {
	evt.payload = payload
}

// Aggregator is main interface that responsibles for event aggregation.
type Aggregator interface {
	GetId() string
	SetId(id string)
	GetType() string
	SetType(typ string)
	GetVersion() Version
	SetVersion(version Version)
	// ListEvents
	ListCommittedEvents() []Eventer
	ListUncommittedEvents() []Eventer
	// Apply
	Apply(event Eventer) error
	ApplyCommitted(event Eventer) error
	// Commit marks provided event as committed and deletes
	// from uncommitted events list.
	Commit(event Eventer) error
}

type Transition func(event Eventer) error
