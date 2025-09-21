package platforms

import (
	"context"
	databaseSql "database/sql"
	gdh "github.com/KoNekoD/gormite/pkg/gormite_databases_helpers"
	"github.com/pkg/errors"
)

type Connection struct {
	db       gdh.Database
	platform AbstractPlatformInterface
}

func NewConnection(db gdh.Database, p AbstractPlatformInterface) *Connection {
	return &Connection{db: db, platform: p}
}

func FetchScan(ctx context.Context, c *Connection, sql string, typedData any) error {
	err := c.db.Select(sql).Scan(typedData).Exec(ctx)
	if err != nil && !errors.Is(err, databaseSql.ErrNoRows) {
		return errors.Wrapf(err, "error when query fetching")
	}

	return nil
}

func (c *Connection) GetDatabasePlatform() AbstractPlatformInterface {
	return c.platform
}

func (c *Connection) FetchAllAssociative(ctx context.Context, sql string) ([]map[string]any, error) {
	result := make([]map[string]any, 0)

	err := c.db.Get(sql).ScanCol(&result).Exec(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when query fetching")
	}

	return result, nil
}

func (c *Connection) GetDatabase(ctx context.Context) (string, error) {
	database := ""

	sql := c.platform.GetDummySelectSQL(c.platform.GetCurrentDatabaseExpression())

	err := c.db.Get(sql).ScanCol(&database).Exec(ctx)
	if err != nil {
		return "", errors.Wrap(err, "error when query fetching get database")
	}

	return database, nil
}
