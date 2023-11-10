package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"

	"github.com/0x9ef/eventsourcing-go/event"
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
		db, err = sql.Open("postgres", fmt.Sprintf("port=%s user=root password=root dbname=test", resourcePort))
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		log.Fatalf("failed to connect to database: %s", err)
	}

	logger.Print("Migration SQL statements...")
	if err := New(db).migrate(context.Background(), createMigrations); err != nil {
		log.Fatalf("failed to migrate: %s", err)
	}

	logger.Print("Running tests...")
	exitCode := m.Run()
	if err := pool.Purge(postgres); err != nil {
		log.Fatalf("failed to purge postgres resource: %s", err)
	}

	logger.Print("Exit.")
	os.Exit(exitCode)
}

type eventTestCreated struct {
	Status string
}

type eventTestConfirmed struct {
	Status string
}

func TestSave(t *testing.T) {
	events := []*event.Event{
		event.MustNew("created", eventTestCreated{Status: "Created"}),
		event.MustNew("confirmed", eventTestConfirmed{Status: "Confirmed"}),
	}

	repo := New(db)
	err := repo.Save(context.TODO(), event.Covarience(events))
	assert.NoError(t, err, "failed to save events in database")
}
