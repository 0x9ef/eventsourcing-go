package postgresql

import (
	"database/sql"
	"log"
	"os"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var db *sql.DB

func TestMain(m *testing.M) {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to init pool: %s", err)
	}
	if err := pool.Client.Ping(); err != nil {
		log.Fatalf("failed to check connection to Docker: %s", err)
	}

	postgres, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "14",
		Env: []string{
			"POSTGRES_USER=root",
			"POSTGRES_PASSWORD=root",
		},
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
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

	if err := pool.Retry(func() error {
		var err error
		db, err = sql.Open("postgres", "port=5433 user=root password=root dbname=postgres")
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		log.Fatalf("failed to connect to database: %s", err)
	}

	exitCode := m.Run()
	if err := pool.Purge(postgres); err != nil {
		log.Fatalf("failed to purge postgres resource: %s", err)
	}
	os.Exit(exitCode)
}
