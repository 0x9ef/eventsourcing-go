package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"

	"github.com/0x9ef/eventsourcing-go"
	"github.com/0x9ef/eventsourcing-go/event"
	"github.com/0x9ef/eventsourcing-go/eventstore"
)

var db *sql.DB

func TestMain(m *testing.M) {
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	logger.Print("Initializing pool...")
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to init pool: %s", err)
	}

	logger.Print("Checking connection to Docker...")
	if err := pool.Client.Ping(); err != nil {
		log.Fatalf("failed to check connection to Docker: %s", err)
	}

	logger.Print("Running resource...")
	postgres, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "14",
		Env: []string{
			"POSTGRES_USER=root",
			"POSTGRES_PASSWORD=root",
			"POSTGRES_DB=test",
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		log.Fatalf("failed to run resource: %s", err)
	}
	if err := postgres.Expire(30); err != nil {
		log.Fatalf("failed to set expire timeout: %s", err)
	}

	resourcePort := postgres.GetPort("5432/tcp")
	logger.Print("Trying to connect to database...")
	if err := pool.Retry(func() error {
		var err error
		db, err = sql.Open("postgres", fmt.Sprintf("port=%s user=root password=root dbname=test sslmode=disable", resourcePort))
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		log.Fatalf("failed to connect to database: %s", err)
	}

	logger.Print("Migration SQL statements...")
	if err := New(db, "es_events").migrate(context.Background(), createMigrations); err != nil {
		log.Fatalf("failed to migrate: %s", err)
	}

	logger.Print("Running tests...")
	exitCode := m.Run()
	if err := pool.Purge(postgres); err != nil {
		log.Fatalf("failed to purge postgres resource: %s", err)
	}

	logger.Printf("Exit %d.", exitCode)
	os.Exit(exitCode)
}

type TestAggregator struct {
	*eventsourcing.AggregateCluster
	Status string
}

const (
	testAggregateReasonCreated   = "created"
	testAggregateReasonConfirmed = "confirmed"
)

func (ta *TestAggregator) Transition(evt event.Eventer) error {
	switch evt.GetReason() {
	case testAggregateReasonCreated:
		return ta.onCreated(evt)
	case testAggregateReasonConfirmed:
		return ta.onConfirmed(evt)
	}
	return errors.New("undefined event type")
}

type eventTestCreated struct {
	Status string
}

func (ta *TestAggregator) onCreated(evt event.Eventer) error {
	var payload eventTestCreated
	if err := json.Unmarshal(evt.GetPayload(), &payload); err != nil {
		return err
	}
	ta.Status = payload.Status
	return nil
}

type eventTestConfirmed struct {
	Status string
}

func (ta *TestAggregator) onConfirmed(evt event.Eventer) error {
	var payload eventTestConfirmed
	if err := json.Unmarshal(evt.GetPayload(), &payload); err != nil {
		return err
	}
	ta.Status = payload.Status
	return nil
}

func TestSave(t *testing.T) {
	agg := &TestAggregator{}
	root := eventsourcing.New(agg, agg.Transition, eventsourcing.NanoidGenerator)

	events := []*event.Event{
		event.MustNew("created", eventTestCreated{Status: "Created"}),
		event.MustNew("confirmed", eventTestConfirmed{Status: "Confirmed"}),
	}
	for _, evt := range events {
		err := root.Apply(evt)
		assert.NoError(t, err, "failed to apply")
	}

	repo := New(db, "es_events")
	err := repo.Save(context.TODO(), event.Covarience(events))
	assert.NoError(t, err, "failed to save events in database")
}

func TestGet(t *testing.T) {
	agg := &TestAggregator{}
	root := eventsourcing.New(agg, agg.Transition, eventsourcing.NanoidGenerator)

	ctx := context.TODO()
	repo := New(db, "es_events")
	events, err := seedEvents(root, repo)
	assert.NoError(t, err, "cannot seed events")

	evt, err := repo.Get(ctx, events[1].GetAggregateId(), events[1].GetAggregateType(), events[1].GetVersion())
	assert.NoError(t, err, "failed to get event from database")
	assert.Equal(t, "TestAggregator", evt.GetAggregateType())
	assert.Equal(t, "confirmed", evt.GetReason())
	assert.Equal(t, event.Version(2), evt.GetVersion())
}

func TestList(t *testing.T) {
	agg := &TestAggregator{}
	root := eventsourcing.New(agg, agg.Transition, eventsourcing.NanoidGenerator)

	ctx := context.TODO()
	repo := New(db, "es_events")
	events, err := seedEvents(root, repo)
	assert.NoError(t, err, "cannot seed events")

	type testCase struct {
		name          string
		aggregateId   string
		aggregateType string
		filter        *eventstore.ListFilter
		// expectations.
		expectedLen int
	}

	cases := []testCase{
		{
			name:          "positive_all",
			aggregateId:   events[0].GetAggregateId(),
			aggregateType: events[0].GetAggregateType(),
			filter:        nil,
			expectedLen:   2,
		},
		{
			name:          "positive_before",
			aggregateId:   events[0].GetAggregateId(),
			aggregateType: events[0].GetAggregateType(),
			filter: &eventstore.ListFilter{
				BeforeVersion: events[1].GetVersion(),
			},
			expectedLen: 1,
		},
		{
			name:          "positive_after",
			aggregateId:   events[0].GetAggregateId(),
			aggregateType: events[0].GetAggregateType(),
			filter: &eventstore.ListFilter{
				AfterVersion: events[0].GetVersion(),
			},
			expectedLen: 1,
		},
		{
			name:          "positive_limit_1",
			aggregateId:   events[0].GetAggregateId(),
			aggregateType: events[0].GetAggregateType(),
			filter: &eventstore.ListFilter{
				Limit: 1,
			},
			expectedLen: 1,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			listEvents, err := repo.List(ctx, testCase.aggregateId, testCase.aggregateType, testCase.filter)
			assert.NoError(t, err, "failed to get list of events")
			assert.Equal(t, testCase.expectedLen, len(listEvents))
		})
	}
}

func seedEvents(root *eventsourcing.AggregateCluster, repo *eventRepository) ([]*event.Event, error) {
	events := []*event.Event{
		event.MustNew("created", eventTestCreated{Status: "Created"}),
		event.MustNew("confirmed", eventTestConfirmed{Status: "Confirmed"}),
	}
	for _, evt := range events {
		err := root.Apply(evt)
		if err != nil {
			return nil, err
		}
	}

	return events, repo.Save(context.TODO(), event.Covarience(events))
}
