package eventsourcinggo

import (
	"errors"
	"reflect"

	"github.com/0x9ef/eventsourcing-go/event"
)

type AggregateRoot struct {
	currentId      string
	currentType    string
	currentVersion event.Version
	// internal.
	committedEvents   []event.Eventer
	uncommittedEvents *linkedList
	transitionfn      event.Transition
}

var _ (event.Aggregator) = &AggregateRoot{}

func New(agg event.Aggregator, transition event.Transition, idgenfn IDGenerator) *AggregateRoot {
	return &AggregateRoot{
		currentId:         idgenfn(idDefaultAlphabet, idDefaultSize),
		currentType:       reflect.TypeOf(agg).Name(),
		committedEvents:   make([]event.Eventer, 0, 8),
		uncommittedEvents: new(linkedList),
		transitionfn:      transition,
	}
}

func (r *AggregateRoot) GetId() string {
	return r.currentId
}

func (r *AggregateRoot) SetId(id string) {
	r.currentId = id
}

func (r *AggregateRoot) GetType() string {
	return r.currentType
}

func (r *AggregateRoot) SetType(typ string) {
	r.currentType = typ
}

func (r *AggregateRoot) GetVersion() event.Version {
	return r.currentVersion
}

func (r *AggregateRoot) SetVersion(version event.Version) {
	r.currentVersion = version
}

// Apply applies not committed yet event. The event Id, Type, Version will
// be replaced with current AggregateRoot Id, Type and Version.
func (r *AggregateRoot) Apply(evt event.Eventer) error {
	return r.apply(evt, false)
}

// ApplyCommitted applies already committed event. The AggregateRoot state
// id, type, version will be replaced with current event id, type and version.
func (r *AggregateRoot) ApplyCommitted(evt event.Eventer) error {
	return r.apply(evt, true)
}

func (r *AggregateRoot) apply(evt event.Eventer, committed bool) error {
	if err := r.transitionfn(evt); err != nil {
		return err
	}

	if committed {
		if err := r.checkVersionDuplication(evt); err != nil {
			return err
		}

		r.currentId = evt.GetAggregateId()
		r.currentType = evt.GetAggregateType()
		r.currentVersion = evt.GetVersion()
		r.committedEvents = append(r.committedEvents, evt)
	} else {
		// Increment our aggregate root version for +1
		r.currentVersion = r.nextVersion()

		evt.SetAggregateId(r.currentId)
		evt.SetAggregateType(r.currentType)
		evt.SetVersion(r.currentVersion)
		r.uncommittedEvents.add(evt)
	}

	return nil
}

// ListCommittedEvents returns a list of already committed events.
func (r *AggregateRoot) ListCommittedEvents() []event.Eventer {
	return r.committedEvents
}

// ListUncommittedEvents returns a list of not committed yet events.
func (r *AggregateRoot) ListUncommittedEvents() []event.Eventer {
	uncommittedEvents := make([]event.Eventer, 0, r.uncommittedEvents.len)
	r.uncommittedEvents.traverse(func(uncommitted event.Eventer) error {
		uncommittedEvents = append(uncommittedEvents, uncommitted)
		return nil
	})
	return uncommittedEvents
}

// Commit commits event and deletes from committed events list.
func (r *AggregateRoot) Commit(evt event.Eventer) error {
	r.uncommittedEvents.remove(evt)
	return nil
}

func (r *AggregateRoot) nextVersion() event.Version {
	return r.currentVersion + event.NextVersion
}

var ErrEventDuplication = errors.New("event duplication, event is already exist")

func (r *AggregateRoot) checkVersionDuplication(evt event.Eventer) error {
	for i := range r.committedEvents {
		committed := r.committedEvents[i]
		if committed.GetAggregateId() == evt.GetAggregateId() &&
			committed.GetAggregateType() == evt.GetAggregateType() &&
			committed.GetVersion() == evt.GetVersion() {

			return ErrEventDuplication
		}
	}
	return nil
}
