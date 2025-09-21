package schema_managers

import (
	"context"
	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/dtos"
)

type AbstractSchemaManagerInterface interface {
	IntrospectSchema(ctx context.Context) (*assets.Schema, error)
	ListTables(ctx context.Context) ([]*assets.Table, error)
	ListSequences(ctx context.Context) ([]*assets.Sequence, error)
	CreateSchemaConfig(ctx context.Context) (*dtos.SchemaConfig, error)
	ListSchemaNames(ctx context.Context) ([]string, error)

	GetPortableDatabaseDefinition(row map[string]any) string
	GetPortableSequenceDefinition(sequence *dtos.ListSequencesDto) *assets.Sequence
	GetPortableTableColumnDefinition(tableColumn *dtos.SelectTableColumnsDto) *assets.Column
	GetPortableViewDefinition(view map[string]any) *assets.View
	GetPortableTableForeignKeyDefinition(tableForeignKey *dtos.SelectForeignKeyColumnsDto) *assets.ForeignKeyConstraint

	GetPortableTableDefinition(ctx context.Context, table dtos.GetPortableTableDefinitionInputDto) (string, error)

	SelectTableNames(ctx context.Context, databaseName string) ([]*dtos.SelectTableNamesDto, error)
	SelectTableColumns(
		ctx context.Context,
		databaseName string,
		tableName *string,
	) ([]*dtos.SelectTableColumnsDto, error)
	SelectIndexColumns(
		ctx context.Context,
		databaseName string,
		tableName *string,
	) ([]*dtos.SelectIndexColumnsDto, error)
	SelectForeignKeyColumns(
		ctx context.Context,
		databaseName string,
		tableName *string,
	) ([]*dtos.SelectForeignKeyColumnsDto, error)

	FetchTableOptionsByTable(
		ctx context.Context,
		databaseName string,
		tableName *string,
	) (map[string]*dtos.FetchTableOptionsByTableDto, error)

	GetPortableTableIndexesList(
		ctx context.Context,
		tableIndexes []*dtos.SelectIndexColumnsDto,
		tableName string,
	) (map[string]*assets.Index, error)
}
