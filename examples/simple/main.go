package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/0x9ef/eventsourcing-go"
	"github.com/0x9ef/eventsourcing-go/event"
	"github.com/0x9ef/eventsourcing-go/eventstore/postgresql"
)

type PaymentAggregator struct {
	*eventsourcing.AggregateCluster
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

func main() {
	// Open connection to the database
	db, err := sql.Open("postgres", "user=root password=root")
	if err != nil {
		panic(err)
	}

	// Define and apply SQL migrations
	migrations := []string{
		`CREATE TABLE public.es_events (
			aggregate_id   VARCHAR(128) NOT NULL,
			aggregate_type VARCHAR(128) NOT NULL,
			reason         TEXT NOT NULL,
			version        SMALLINT NOT NULL,
			tstamp         TIMESTAMPTZ NOT NULL,
			payload        bytea,
			serializer     VARCHAR(16)
		);`,
		"CREATE UNIQUE INDEX id_type_version_un ON public.es_events (aggregate_id, aggregate_type, version);",
		"CREATE INDEX id_type_idx ON public.es_events (aggregate_id, aggregate_type);",
	}

	ctx := context.TODO()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	for _, stmt := range migrations {
		if _, err := tx.Exec(stmt); err != nil {
			panic(err)
		}
	}
	if err := tx.Commit(); err != nil {
		panic(err)
	}

	// Create our PaymentAggregator cluster
	agg := &PaymentAggregator{}
	agg.AggregateCluster = eventsourcing.New(agg, agg.Transition, eventsourcing.UUIDGenerator)

	// Our sequences events
	created, _ := event.New(PaymentAggregateReasonCreated, paymentCreatedEvent{
		PaymentID:              "id_0",
		PaymentStatus:          "created",
		PaymentAmount:          100,
		PaymentAvailableAmount: 100,
	})
	confirmed, _ := event.New(PaymentAggregateReasonConfirmed, paymentConfirmedEvent{
		PaymentStatus: "confirmed",
	})
	refunded, _ := event.New(PaymentAggregateReasonRefunded, paymentRefundEvent{
		PaymentRefundAmount: 50,
	})

	events := []*event.Event{created, confirmed, refunded}
	for _, evt := range events {
		// Apply events to our aggregate cluster
		if err := agg.Apply(evt); err != nil {
			panic(err)
		}
	}

	repo := postgresql.New(db, "es_events")
	if err := repo.Save(ctx, event.Covarience(events)); err != nil {
		panic(err)
	}
}
