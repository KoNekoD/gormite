package gormite_databases

import gdh "github.com/KoNekoD/gormite/pkg/gormite_databases_helpers"

type QueryError struct {
	err   error
	query gdh.QueryInterface
}

func (e *QueryError) Error() string {
	return e.err.Error()
}

func (e *QueryError) Unwrap() error {
	return e.err
}

type SqlError struct {
	err  error
	sql  string
	args []any
}

func NewSqlError(err error, sql string, args []any) *SqlError {
	return &SqlError{err: err, sql: sql, args: args}
}

func (e *SqlError) Error() string {
	return e.err.Error()
}

func (e *SqlError) GetExtraData() map[string]any {
	return map[string]any{
		"sql":  e.sql,
		"args": e.args,
	}
}

func (e *SqlError) Unwrap() error {
	return e.err
}
