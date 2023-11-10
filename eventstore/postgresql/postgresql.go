package postgresql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/0x9ef/eventsourcing-go/event"
	"github.com/huandu/go-sqlbuilder"
)

type eventRepository struct {
	conn *sql.DB
}

func (r *eventRepository) Get(ctx context.Context, aggregateID, aggregateType string, version event.Version) (event.Eventer, error) {
	b := sqlbuilder.PostgreSQL.
		NewSelectBuilder().
		Select(
			"aggregateId",
			"aggregateType",
			"version",
			"tstamp",
			"payload",
		).
		From("es_events")

	b = b.Where(
		b.Equal("aggregate_id", aggregateID),
		b.And(
			b.Equal("aggregate_type", aggregateType),
			b.Equal("version", version),
		),
	)

	q, args := b.Build()

	var (
		evtAggregateId   string
		evtAggregateType string
		evtVersion       event.Version
		evtTimestamp     event.Timestamp
		evtPayload       event.Payload
	)

	err := r.conn.
		QueryRowContext(ctx, q, args).
		Scan(
			&evtAggregateId,
			&evtAggregateType,
			&evtVersion,
			&evtTimestamp,
			&evtPayload,
		)
	if err != nil {
		return nil, err
	}

	evt := new(event.Event)
	evt.SetAggregateId(evtAggregateId)
	evt.SetAggregateType(evtAggregateType)
	evt.SetVersion(evtVersion)
	evt.SetTimestamp(evtTimestamp)
	evt.SetPayload(evtPayload)

	return evt, nil
}

type ListFilter struct {
	AfterVersion  event.Version
	BeforeVersion event.Version
	Limit         int
}

func (r *eventRepository) List(ctx context.Context, aggregateID, aggregateType string, filter *ListFilter) ([]event.Eventer, error) {
	b := sqlbuilder.PostgreSQL.
		NewSelectBuilder().
		Select(
			"aggregate_id",
			"aggregate_type",
			"version",
			"tstamp",
			"payload",
		).
		From("es_events")

	var whereExpr []string
	whereExpr = append(whereExpr, b.Equal("aggregate_id", aggregateID))
	whereExpr = append(whereExpr, b.And(b.Equal("aggregate_type", aggregateType)))
	if filter != nil && filter.BeforeVersion > 0 {
		whereExpr = append(whereExpr, b.And(b.LessThan("version", filter.BeforeVersion)))
	}
	if filter != nil && filter.AfterVersion > 0 {
		whereExpr = append(whereExpr, b.And(b.GreaterThan("version", filter.AfterVersion)))
	}

	b = b.Where(whereExpr...)
	if filter != nil && filter.Limit > 0 {
		b = b.Limit(filter.Limit)
	}

	q, args := b.Build()
	rows, err := r.conn.QueryContext(ctx, q, args)
	if err != nil {
		return nil, err
	}

	var rowsSize = 16 // preallocated buffer
	if filter.Limit > 0 {
		rowsSize = filter.Limit
	}
	events := make([]event.Eventer, 0, rowsSize)
	for rows.Next() {
		var (
			aggregateId   string
			aggregateType string
			version       event.Version
			tstamp        event.Timestamp
			payload       event.Payload
		)
		if err := rows.Scan(&aggregateId, &aggregateType, &version, &tstamp, &payload); err != nil {
			return nil, err
		}

		evt := new(event.Event)
		evt.SetAggregateId(aggregateId)
		evt.SetAggregateType(aggregateType)
		evt.SetVersion(version)
		evt.SetTimestamp(tstamp)
		evt.SetPayload(payload)
		events = append(events, evt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (r *eventRepository) Save(ctx context.Context, events []event.Eventer) error {
	if len(events) == 0 {
		return nil
	}

	aggregateId := events[0].GetAggregateId()
	aggregateType := events[0].GetAggregateType()
	version := events[0].GetVersion()

	// Begin transaction in default mode
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Try to control concurrency
	if err := r.controlConcurrency(ctx, tx, aggregateId, aggregateType, version); err != nil {
		return err
	}

	ib := sqlbuilder.PostgreSQL.
		NewInsertBuilder().
		InsertInto("es_events").
		Cols(
			"aggregate_id",
			"aggregate_type",
			"version",
			"tstamp",
			"payload",
		)

	for _, evt := range events {
		ib = ib.Values(
			evt.GetAggregateId(),
			evt.GetAggregateType(),
			evt.GetVersion(),
			evt.GetTimestamp(),
			evt.GetPayload(),
		)
		q, args := ib.Build()

		if _, err := r.conn.Exec(q, args); err != nil {
			return err
		}
	}

	return tx.Commit()
}

var ErrControlConcurrency = errors.New("concurrency error")

// Optimistic concurrency control
// https://en.wikipedia.org/wiki/Optimistic_concurrency_control
func (r *eventRepository) controlConcurrency(ctx context.Context, tx *sql.Tx, aggregateId, aggregateType string, version event.Version) error {
	sb := sqlbuilder.PostgreSQL.
		NewSelectBuilder().
		Select("version").
		From("es_events")

	sb = sb.Where(
		sb.Equal("aggregate_id", aggregateId),
		sb.And(
			sb.Equal("aggregate_type", aggregateType),
		),
	)
	sb = sb.
		OrderBy("version").
		Desc().
		Limit(1)

	sbq, sbargs := sb.Build()

	var lastAggregateVersion event.Version
	err := tx.QueryRowContext(ctx, sbq, sbargs...).Scan(&lastAggregateVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			lastAggregateVersion = event.EmptyVersion
		}
		return err
	}

	// Check that no other versions are inserted
	if lastAggregateVersion+event.NextVersion != version {
		return ErrControlConcurrency
	}

	return nil
}
