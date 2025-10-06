package gormite_databases

import (
	"context"
	databaseSql "database/sql"
	gdh "github.com/KoNekoD/gormite/pkg/gormite_databases_helpers"
	"github.com/KoNekoD/pgx-colon-query-rewriter/pkg/pgxcqr"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"log"
)

type PostgresOptionFn func(o *Postgres)

func PostgresWithOnError(onError func(method string, err error, sql string, args ...any)) PostgresOptionFn {
	return func(o *Postgres) { o.onError = onError }
}

type PgXWrappedDatabase interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Postgres struct {
	pgx       PgXWrappedDatabase
	pgxConfig *pgxpool.Config

	pgxConn *pgxpool.Pool
	onError func(method string, err error, sql string, args ...any)
}

func NewPostgres(ctx context.Context, dsn string, opts ...PostgresOptionFn) *Postgres {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("Cannot parse config: %v\n", err)
	}

	pgxPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	onError := func(method string, err error, sql string, args ...any) {
		log.Printf("query error %v, sql %s, args %v\n", err, sql, args)
	}

	v := &Postgres{pgx: pgxPool, pgxConfig: config, pgxConn: pgxPool, onError: onError}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

func (d *Postgres) WrapInTransaction(ctx context.Context, fn func(ctx context.Context, db *Postgres) error) error {
	opts := pgx.TxOptions{IsoLevel: pgx.ReadCommitted}

	var (
		tx  pgx.Tx
		err error
	)

	switch p := d.pgx.(type) {
	case *pgxpool.Pool:
		tx, err = p.BeginTx(ctx, opts)
	case pgx.Tx:
		tx, err = p.Begin(ctx)
	}

	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}

	db := &Postgres{pgx: tx, pgxConfig: d.pgxConfig, pgxConn: d.pgxConn, onError: d.onError}

	if err = fn(ctx, db); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return multierror.Append(err, errors.Wrap(rollbackErr, "failed to rollback transaction"))
		}

		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

func (d *Postgres) Select(sql string, args ...any) gdh.QueryInterface {
	return &PostgresQuery{db: d.pgx, sql: sql, args: args, onError: d.onError}
}

func (d *Postgres) Get(sql string, args ...any) gdh.QueryInterface {
	return &PostgresQuery{db: d.pgx, sql: sql, args: args, scanFirst: true, onError: d.onError}
}

func (d *Postgres) Exec(ctx context.Context, sql string, args ...any) (gdh.CommandTag, error) {
	tag, err := d.pgx.Exec(ctx, sql, args...)

	if err != nil && !errors.Is(err, databaseSql.ErrNoRows) {
		d.onError("Exec", err, trimSQL(sql), args...)
	}

	return tag, errors.WithStack(err)
}

func (d *Postgres) Query(ctx context.Context, sql string, args ...any) (gdh.Rows, error) {
	rows, err := d.pgx.Query(ctx, sql, args...)

	if err != nil && !errors.Is(err, databaseSql.ErrNoRows) {
		d.onError("Query", err, trimSQL(sql), args...)
	}

	return rows, errors.WithStack(err)
}

func (d *Postgres) GetNamedArgs(args any) any {
	return pgxcqr.NamedArgs(args.(map[string]any))
}

func (d *Postgres) GetPgx() PgXWrappedDatabase {
	return d.pgx
}

func (d *Postgres) GetPgxConfig() *pgxpool.Config {
	return d.pgxConfig
}

func (d *Postgres) Destruct() {
	d.pgxConn.Close()
}
