package eventsourcing

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/0x9ef/eventsourcing-go/event"
	"github.com/stretchr/testify/assert"
)

type PaymentAggregator struct {
	*AggregateRoot
	// General.
	PaymentID              string
	PaymentStatus          string
	PaymentAmount          int
	PaymentAvailableAmount int
	PaymentRefundAmount    int
}

const (
	PaymentAggregateReasonCreated   = "created"
	PaymentAggregateReasonConfirmed = "confirmed"
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
	return errors.New("undefined event type")
}

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

func TestApply(t *testing.T) {
	agg := &PaymentAggregator{}
	root := New(agg, agg.Transition, NanoidGenerator)

	events := []*event.Event{
		mustNewEvent(PaymentAggregateReasonCreated, paymentCreatedEvent{
			PaymentID:              "id_0",
			PaymentStatus:          "created",
			PaymentAmount:          100,
			PaymentAvailableAmount: 100,
		}),
		mustNewEvent(PaymentAggregateReasonConfirmed, paymentConfirmedEvent{
			PaymentStatus: "confirmed",
		}),
		mustNewEvent(PaymentAggregateReasonRefunded, paymentRefundEvent{
			PaymentRefundAmount: 50,
		}),
	}

	for _, evt := range events {
		err := root.Apply(evt)
		assert.NoError(t, err, "cannot apply event")
	}

	assert.Equal(t, "id_0", agg.PaymentID)
	assert.Equal(t, "confirmed", agg.PaymentStatus)
	assert.Equal(t, 100, agg.PaymentAmount)
	assert.Equal(t, 50, agg.PaymentRefundAmount)
	assert.Equal(t, 50, agg.PaymentAvailableAmount)
	assert.Equal(t, event.Version(3), root.GetVersion())
}

func mustNewEvent(reason string, payload interface{}) *event.Event {
	e, err := event.New(reason, payload)
	if err != nil {
		panic(err)
	}
	return e
}

func TestApplyCommitted(t *testing.T) {
	agg := &PaymentAggregator{}
	root := New(agg, agg.Transition, NanoidGenerator)

	evtCreated := mustNewEvent(PaymentAggregateReasonCreated, paymentCreatedEvent{
		PaymentID:              "id_0",
		PaymentStatus:          "created",
		PaymentAmount:          100,
		PaymentAvailableAmount: 100,
	})
	evtCreated.SetAggregateId("agg_0")
	evtCreated.SetAggregateType("PaymentAggregator")
	evtCreated.SetVersion(1)

	evtConfirmed := mustNewEvent(PaymentAggregateReasonConfirmed, paymentConfirmedEvent{
		PaymentStatus: "confirmed",
	})
	evtConfirmed.SetAggregateId("agg_0")
	evtConfirmed.SetAggregateType("PaymentAggregator")
	evtConfirmed.SetVersion(2)

	evtRefunded := mustNewEvent(PaymentAggregateReasonRefunded, paymentRefundEvent{
		PaymentRefundAmount: 50,
	})
	evtRefunded.SetAggregateId("agg_0")
	evtRefunded.SetAggregateType("PaymentAggregator")
	evtRefunded.SetVersion(3)

	for _, evt := range []*event.Event{evtCreated, evtConfirmed, evtRefunded} {
		err := root.ApplyCommitted(evt)
		assert.NoError(t, err, "failed to apply committed")
	}

	assert.Equal(t, "id_0", agg.PaymentID)
	assert.Equal(t, "confirmed", agg.PaymentStatus)
	assert.Equal(t, 100, agg.PaymentAmount)
	assert.Equal(t, 50, agg.PaymentRefundAmount)
	assert.Equal(t, 50, agg.PaymentAvailableAmount)
	assert.Equal(t, "agg_0", root.GetId(), "mismatched aggregate id")
	assert.Equal(t, "PaymentAggregator", root.GetType(), "mismatched aggregate type")
	assert.Equal(t, event.Version(3), root.GetVersion(), "mismatched event version")
}

func TestApplyCommittedDuplicate(t *testing.T) {
	agg := &PaymentAggregator{}
	root := New(agg, agg.Transition, NanoidGenerator)

	evtCreated := mustNewEvent(PaymentAggregateReasonCreated, paymentCreatedEvent{
		PaymentID:              "id_0",
		PaymentStatus:          "created",
		PaymentAmount:          100,
		PaymentAvailableAmount: 100,
	})
	evtCreated.SetAggregateId("agg_0")
	evtCreated.SetAggregateType("PaymentAggregator")
	evtCreated.SetVersion(1)

	evtConfirmed := mustNewEvent(PaymentAggregateReasonConfirmed, paymentConfirmedEvent{
		PaymentStatus: "confirmed",
	})
	evtConfirmed.SetAggregateId("agg_0")
	evtConfirmed.SetAggregateType("PaymentAggregator")
	evtConfirmed.SetVersion(1) // version duplication

	for _, evt := range []*event.Event{evtCreated, evtConfirmed} {
		err := root.ApplyCommitted(evt)
		if err != nil {
			assert.Error(t, err, ErrEventDuplication)
		}
	}
}

func TestCommit(t *testing.T) {
	agg := &PaymentAggregator{}
	root := New(agg, agg.Transition, NanoidGenerator)

	events := []*event.Event{
		mustNewEvent(PaymentAggregateReasonCreated, paymentCreatedEvent{
			PaymentID:              "id_0",
			PaymentStatus:          "created",
			PaymentAmount:          100,
			PaymentAvailableAmount: 100,
		}),
		mustNewEvent(PaymentAggregateReasonConfirmed, paymentConfirmedEvent{
			PaymentStatus: "confirmed",
		}),
		mustNewEvent(PaymentAggregateReasonRefunded, paymentRefundEvent{
			PaymentRefundAmount: 50,
		}),
	}

	for _, evt := range events {
		err := root.Commit(evt)
		assert.NoError(t, err, "failed to commit")
	}
	assert.Equal(t, 0, len(root.committedEvents))
}
