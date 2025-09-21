package runners

import (
	"context"
	"fmt"
	"github.com/KoNekoD/gormite/pkg/diff_calc"
	"github.com/KoNekoD/gormite/pkg/gormite_databases"
	"github.com/KoNekoD/gormite/pkg/local_schema"
	"github.com/KoNekoD/gormite/pkg/platforms"
	"github.com/KoNekoD/gormite/pkg/platforms/postgres_platform"
	"github.com/KoNekoD/gormite/pkg/schema_managers/postgres_schema_manager"
	"github.com/pkg/errors"
	"log"
	"os"
	"time"
)

const GooseMigrationTemplate = `-- +goose Up
-- +goose StatementBegin
%s
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
%s
-- +goose StatementEnd
`

type MigrationToolType string

const (
	MigrationToolTypeGoose   MigrationToolType = "goose"
	MigrationToolTypeMigrate MigrationToolType = "migrate"
)

const (
	ScenarioTypeDiff     = "diff"
	ScenarioTypeValidate = "validate"
)

type DiffRunnerOptions struct {
	Tool       string
	Dsn        string
	ConfigPath string
	Scenario   string
}

type DiffRunner struct{ opts DiffRunnerOptions }

func NewDiffRunner(opts DiffRunnerOptions) *DiffRunner {
	return &DiffRunner{opts: opts}
}

func (r *DiffRunner) Run(ctx context.Context) error {
	db := gormite_databases.NewPostgresDatabase(ctx, r.opts.Dsn)

	platform := postgres_platform.NewPostgreSQLPlatform()

	manager := postgres_schema_manager.NewPostgreSQLSchemaManager(platforms.NewConnection(db, platform), platform)

	oldSchema, err := manager.IntrospectSchema(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to introspect remote schema")
	}

	newSchema, err := local_schema.IntrospectLocalSchema(r.opts.ConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to introspect local schema")
	}

	c := diff_calc.NewComparator(platform)

	diff := c.CompareSchemas(oldSchema, newSchema)
	diffDown := c.CompareSchemas(newSchema, oldSchema)

	switch r.opts.Scenario {
	case ScenarioTypeDiff:
		if diff.IsEmpty() {
			return errors.New("No changes detected")
		}

		up := manager.AlterSchema(diff)
		down := manager.AlterSchema(diffDown)

		switch r.opts.Tool {
		case string(MigrationToolTypeMigrate):
			key := time.Now().Format("20060102150405")
			file := fmt.Sprintf("migrations/%s_gen.up.sql", key)
			fileDown := fmt.Sprintf("migrations/%s_gen.down.sql", key)
			if err := os.WriteFile(file, []byte(up), 0644); err != nil {
				return errors.Wrap(err, "Cannot write up sql file")
			}
			if err := os.WriteFile(fileDown, []byte(down), 0644); err != nil {
				return errors.Wrap(err, "Cannot write down sql file")
			}
		case string(MigrationToolTypeGoose):
			key := time.Now().Format("20060102150405")
			file := fmt.Sprintf("migrations/%s_gen.sql", key)
			if err := os.WriteFile(
				file,
				[]byte(fmt.Sprintf(GooseMigrationTemplate, up, down)),
				0644,
			); err != nil {
				return errors.Wrap(err, "Cannot write migration file")
			}
		}
	case ScenarioTypeValidate:
		if diff.IsEmpty() {
			log.Println("The database schema is in sync with the mapping files.")
			return nil
		}

		sqlList := manager.AlterSchemaSqlList(diff)

		log.Println("The database schema is not in sync with the current mapping file.")

		log.Printf("%d schema diff(s) detected:\n", len(sqlList))
		for _, sql := range sqlList {
			log.Printf("    %s\n", sql)
		}

		return errors.New("The database schema is not in sync with the current mapping file.")
	}

	return nil
}
