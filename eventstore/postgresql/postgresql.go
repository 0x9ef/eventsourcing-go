package postgresql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/huandu/go-sqlbuilder"

	"github.com/0x9ef/eventsourcing-go/event"
	"github.com/0x9ef/eventsourcing-go/eventstore"
)

type eventRepository struct {
	tableName string
	conn      *sql.DB
}

func New(conn *sql.DB, tableName string) *eventRepository {
	return &eventRepository{tableName: tableName, conn: conn}
}

func (r *eventRepository) Get(ctx context.Context, aggregateID, aggregateType string, version event.Version) (event.Eventer, error) {
	sb := sqlbuilder.PostgreSQL.
		NewSelectBuilder().
		Select(
			"aggregate_id",
			"aggregate_type",
			"reason",
			"version",
			"tstamp",
			"payload",
		).
		From(r.tableName)

	sb = sb.Where(
		sb.Equal("aggregate_id", aggregateID),
		sb.And(
			sb.Equal("aggregate_type", aggregateType),
			sb.Equal("version", version),
		),
	)

	q, args := sb.Build()

	var (
		evtAggregateId   string
		evtAggregateType string
		evtReason        string
		evtVersion       event.Version
		evtTimestamp     event.Timestamp
		evtPayload       event.Payload
	)

	err := r.conn.
		QueryRowContext(ctx, q, args...).
		Scan(
			&evtAggregateId,
			&evtAggregateType,
			&evtReason,
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
	evt.SetReason(evtReason)
	evt.SetVersion(evtVersion)
	evt.SetTimestamp(evtTimestamp)
	evt.SetPayload(evtPayload)

	return evt, nil
}

func (r *eventRepository) List(ctx context.Context, aggregateID, aggregateType string, filter *eventstore.ListFilter) ([]event.Eventer, error) {
	sb := sqlbuilder.PostgreSQL.
		NewSelectBuilder().
		Select(
			"aggregate_id",
			"aggregate_type",
			"reason",
			"version",
			"tstamp",
			"payload",
		).
		From(r.tableName)

	var whereExpr []string
	whereExpr = append(whereExpr, sb.Equal("aggregate_id", aggregateID))
	whereExpr = append(whereExpr, sb.And(sb.Equal("aggregate_type", aggregateType)))
	if filter != nil && filter.BeforeVersion > 0 {
		whereExpr = append(whereExpr, sb.And(sb.LessThan("version", filter.BeforeVersion)))
	}
	if filter != nil && filter.AfterVersion > 0 {
		whereExpr = append(whereExpr, sb.And(sb.GreaterThan("version", filter.AfterVersion)))
	}

	sb = sb.Where(whereExpr...)
	if filter != nil && filter.Limit > 0 {
		sb = sb.Limit(filter.Limit)
	}

	q, args := sb.Build()
	rows, err := r.conn.QueryContext(ctx, q, args...)
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
			reason        string
			version       event.Version
			tstamp        event.Timestamp
			payload       event.Payload
		)
		if err := rows.Scan(&aggregateId, &aggregateType, &reason, &version, &tstamp, &payload); err != nil {
			return nil, err
		}

		evt := new(event.Event)
		evt.SetAggregateId(aggregateId)
		evt.SetAggregateType(aggregateType)
		evt.SetReason(reason)
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

	for _, evt := range events {
		ib := sqlbuilder.PostgreSQL.
			NewInsertBuilder().
			InsertInto(r.tableName).
			Cols(
				"aggregate_id",
				"aggregate_type",
				"reason",
				"version",
				"tstamp",
				"payload",
			)

		ib = ib.Values(
			evt.GetAggregateId(),
			evt.GetAggregateType(),
			evt.GetReason(),
			evt.GetVersion(),
			evt.GetTimestamp(),
			evt.GetPayload(),
		)
		q, args := ib.Build()

		if _, err := r.conn.Exec(q, args...); err != nil {
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
		From(r.tableName)

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

	q, args := sb.Build()

	var lastAggregateVersion event.Version
	err := tx.QueryRowContext(ctx, q, args...).Scan(&lastAggregateVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			lastAggregateVersion = event.EmptyVersion
		} else {
			return err
		}
	}

	// Check that no other versions are inserted
	if (lastAggregateVersion + event.NextVersion) != version {
		return ErrControlConcurrency
	}

	return nil
}
