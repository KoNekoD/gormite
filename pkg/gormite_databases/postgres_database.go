package gormite_databases

import (
	"context"
	databaseSql "database/sql"
	gdh "github.com/KoNekoD/gormite/pkg/gormite_databases_helpers"
	"github.com/KoNekoD/pgx-colon-query-rewriter/pkg/pgxcqr"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"log"
)

type PostgresOptionFn func(o *PostgresDatabase)

func PostgresWithOnError(onError func(method string, err error, sql string, args ...any)) PostgresOptionFn {
	return func(o *PostgresDatabase) { o.onError = onError }
}

type PostgresDatabase struct {
	pgx       *pgxpool.Pool
	pgxConfig *pgxpool.Config

	onError func(method string, err error, sql string, args ...any)
}

func NewPostgresDatabase(ctx context.Context, dsn string, opts ...PostgresOptionFn) *PostgresDatabase {
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

	v := &PostgresDatabase{pgx: pgxPool, pgxConfig: config, onError: onError}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

func (d *PostgresDatabase) WrapInTransaction(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	opts := pgx.TxOptions{IsoLevel: pgx.ReadCommitted}

	tx, err := d.pgx.BeginTx(ctx, opts)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}

	if err = fn(ctx, tx); err != nil {
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

func (d *PostgresDatabase) Select(sql string, args ...any) gdh.QueryInterface {
	return &PostgresQuery{db: d.pgx, sql: sql, args: args, onError: d.onError}
}

func (d *PostgresDatabase) Get(sql string, args ...any) gdh.QueryInterface {
	return &PostgresQuery{db: d.pgx, sql: sql, args: args, scanFirst: true, onError: d.onError}
}

func (d *PostgresDatabase) Exec(ctx context.Context, sql string, args ...any) (gdh.CommandTag, error) {
	tag, err := d.pgx.Exec(ctx, sql, args...)

	if err != nil && !errors.Is(err, databaseSql.ErrNoRows) {
		d.onError("Exec", err, trimSQL(sql), args...)
	}

	return tag, errors.WithStack(err)
}

func (d *PostgresDatabase) Query(ctx context.Context, sql string, args ...any) (gdh.Rows, error) {
	rows, err := d.pgx.Query(ctx, sql, args...)

	if err != nil && !errors.Is(err, databaseSql.ErrNoRows) {
		d.onError("Query", err, trimSQL(sql), args...)
	}

	return rows, errors.WithStack(err)
}

func (d *PostgresDatabase) GetNamedArgs(args any) any {
	return pgxcqr.NamedArgs(args.(map[string]any))
}

func (d *PostgresDatabase) GetPgx() *pgxpool.Pool {
	return d.pgx
}

func (d *PostgresDatabase) GetPgxConfig() *pgxpool.Config {
	return d.pgxConfig
}

func (d *PostgresDatabase) Destruct() {
	d.pgx.Close()
}
