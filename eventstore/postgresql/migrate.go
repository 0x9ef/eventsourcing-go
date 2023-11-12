package postgresql

import "context"

var createMigrations = []string{
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

func (r *eventRepository) migrate(ctx context.Context, stmts []string) error {
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}
