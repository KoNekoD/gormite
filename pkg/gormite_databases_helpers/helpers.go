package gdh

import (
	"context"
	databaseSql "database/sql"

	"github.com/pkg/errors"
)

type CommandTag interface {
	RowsAffected() int64
	String() string
	Insert() bool
	Update() bool
	Delete() bool
	Select() bool
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
}

type QueryInterface interface {
	Scan(dest ...any) QueryInterface
	ScanCol(dest ...any) QueryInterface
	Exec(ctx context.Context) error
}

type Database interface {
	Select(sql string, args ...any) QueryInterface
	Get(sql string, args ...any) QueryInterface
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	GetNamedArgs(args any) any
}

func SelectExec[T any](ctx context.Context, db Database, sql string, args ...any) (*T, error) {
	var v T

	err := db.Select(sql, args...).Scan(&v).Exec(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &v, nil
}

func SelectExecSlice[T any](ctx context.Context, db Database, sql string, args ...any) ([]*T, error) {
	var v []*T

	err := db.Select(sql, args...).Scan(&v).Exec(ctx)
	if err != nil && !errors.Is(err, databaseSql.ErrNoRows) {
		return nil, errors.WithStack(err)
	}

	return v, nil
}

func SelectExecLit[T any](ctx context.Context, db Database, sql string, args ...any) (T, error) {
	var v T

	err := db.Select(sql, args...).ScanCol(&v).Exec(ctx)
	if err != nil {
		return v, errors.WithStack(err)
	}

	return v, nil
}

func SelectExecScanLit[T any](ctx context.Context, db Database, sql string, args ...any) (T, error) {
	var v T

	err := db.Select(sql, args...).ScanCol(&v).Exec(ctx)
	if err != nil {
		return v, errors.WithStack(err)
	}

	return v, nil
}

func SelectExecLitSlice[T any](ctx context.Context, db Database, sql string, args ...any) ([]T, error) {
	var v []T

	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for rows.Next() {
		var rowValue T

		if err := rows.Scan(&rowValue); err != nil {
			return nil, errors.WithStack(err)
		}

		v = append(v, rowValue)
	}

	return v, nil
}

func Exec(ctx context.Context, db Database, sql string, args ...any) error {
	_, err := db.Exec(ctx, sql, args...)

	return errors.WithStack(err)
}
