package abstract_schema_managers

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/diff_calc"
	"github.com/KoNekoD/gormite/pkg/diff_dtos"
	"github.com/KoNekoD/gormite/pkg/dtos"
	"github.com/KoNekoD/gormite/pkg/platforms"
	"github.com/KoNekoD/gormite/pkg/schema_managers"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
)

type AbstractSchemaManager struct {
	Connection *platforms.Connection

	Platform platforms.AbstractPlatformInterface

	Child schema_managers.AbstractSchemaManagerInterface
}

func NewAbstractSchemaManager(
	connection *platforms.Connection,
	platform platforms.AbstractPlatformInterface,
) *AbstractSchemaManager {
	return &AbstractSchemaManager{Connection: connection, Platform: platform}
}

func (m *AbstractSchemaManager) ListDatabases(ctx context.Context) ([]string, error) {
	items, err := m.Connection.FetchAllAssociative(ctx, m.Platform.GetListDatabasesSQL())
	if err != nil {
		return nil, errors.Wrap(err, "error when listing databases")
	}

	result := make([]string, 0)

	for _, item := range items {
		result = append(result, m.Child.GetPortableDatabaseDefinition(item))
	}

	return result, nil
}

func (m *AbstractSchemaManager) ListSchemaNames() []string {
	panic("not implemented")
}

func (m *AbstractSchemaManager) ListSequences(ctx context.Context) ([]*assets.Sequence, error) {
	items := make([]dtos.ListSequencesDto, 0)

	database, err := m.getDatabase(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting database")
	}

	err = platforms.FetchScan(ctx, m.Connection, m.Platform.GetListSequencesSQL(database), &items)
	if err != nil {
		return nil, errors.Wrap(err, "error when listing sequences")
	}

	result := make([]*assets.Sequence, 0)

	for _, item := range items {
		result = append(result, m.Child.GetPortableSequenceDefinition(&item))
	}

	return result, nil
}

func (m *AbstractSchemaManager) ListTableColumns(ctx context.Context, table string) (map[string]*assets.Column, error) {
	name := m.normalizeName(table)

	database, err := m.getDatabase(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting database")
	}

	columns, err := m.Child.SelectTableColumns(ctx, database, &name)
	if err != nil {
		return nil, errors.Wrap(err, "error when listing table columns")
	}

	return m.GetPortableTableColumnList(table, database, columns), nil
}

func (m *AbstractSchemaManager) ListTableIndexes(ctx context.Context, table string) (map[string]*assets.Index, error) {
	database, err := m.getDatabase(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting database")
	}

	table = m.normalizeName(table)

	columns, err := m.Child.SelectIndexColumns(ctx, database, &table)
	if err != nil {
		return nil, errors.Wrap(err, "error when listing table indexes")
	}

	return m.Child.GetPortableTableIndexesList(ctx, columns, table)
}

func (m *AbstractSchemaManager) TablesExist(ctx context.Context, names []string) (bool, error) {
	lowerSearchTables := make([]string, len(names))
	for i, name := range names {
		lowerSearchTables[i] = strings.ToLower(name)
	}

	databaseTables, err := m.ListTableNames(ctx)
	if err != nil {
		return false, errors.Wrap(err, "error when listing table names")
	}
	lowerDatabaseTables := make([]string, len(databaseTables))
	for i, name := range databaseTables {
		lowerDatabaseTables[i] = strings.ToLower(name)
	}

	for _, databaseTable := range lowerDatabaseTables {
		found := false

		for _, searchTable := range lowerSearchTables {
			if databaseTable == searchTable {
				found = true
				break
			}
		}

		if !found {
			return false, nil
		}
	}

	return true, nil
}

func (m *AbstractSchemaManager) tableExists(ctx context.Context, tableName string) (bool, error) {
	return m.TablesExist(ctx, []string{tableName})
}

func (m *AbstractSchemaManager) ListTableNames(ctx context.Context) ([]string, error) {
	database, err := m.getDatabase(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting database")
	}

	names, err := m.Child.SelectTableNames(ctx, database)
	if err != nil {
		return nil, errors.Wrap(err, "error when listing table names")
	}

	result := make([]string, 0)

	for _, name := range names {
		tableName, err := m.Child.GetPortableTableDefinition(ctx, name)
		if err != nil {
			return nil, errors.Wrap(err, "error when getting portable table definition")
		}

		result = append(result, tableName)
	}

	return result, nil
}

func (m *AbstractSchemaManager) ListTables(ctx context.Context) ([]*assets.Table, error) {
	database, err := m.getDatabase(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting database")
	}

	tableColumnsByTable, err := m.fetchTableColumnsByTable(ctx, database)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching table columns by table")
	}
	indexColumnsByTable, err := m.fetchIndexColumnsByTable(ctx, database)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching index columns by table")
	}
	foreignKeyColumnsByTable, err := m.fetchForeignKeyColumnsByTable(ctx, database)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching foreign key columns by table")
	}
	tableOptionsByTable, err := m.Child.FetchTableOptionsByTable(ctx, database, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching table options by table")
	}

	tables := make([]*assets.Table, 0)
	for tableName, tableColumns := range tableColumnsByTable {
		options := tableOptionsByTable[tableName]

		indexes, err := m.Child.GetPortableTableIndexesList(ctx, indexColumnsByTable[tableName], tableName)
		if err != nil {
			return nil, errors.Wrap(err, "error when getting portable table indexes list")
		}

		tables = append(
			tables, assets.NewTable(
				tableName,
				maps.Values(m.GetPortableTableColumnList(tableName, database, tableColumns)),
				maps.Values(indexes),
				make([]*assets.UniqueConstraint, 0),
				m.getPortableTableForeignKeysList(foreignKeyColumnsByTable[tableName]),
				options.ToArray(),
			),
		)
	}

	return tables, nil
}

func (m *AbstractSchemaManager) normalizeName(name string) string {
	identifier := assets.NewIdentifier(name)

	return identifier.GetName()
}

func (m *AbstractSchemaManager) fetchTableColumnsByTable(
	ctx context.Context,
	databaseName string,
) (map[string][]*dtos.SelectTableColumnsDto, error) {
	data, err := m.Child.SelectTableColumns(ctx, databaseName, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching table columns by table")
	}

	return fetchAllColumns(ctx, m, data)
}

func (m *AbstractSchemaManager) fetchIndexColumnsByTable(
	ctx context.Context,
	databaseName string,
) (map[string][]*dtos.SelectIndexColumnsDto, error) {
	data, err := m.Child.SelectIndexColumns(ctx, databaseName, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching index columns by table")
	}

	return fetchAllColumns(ctx, m, data)
}

func (m *AbstractSchemaManager) fetchForeignKeyColumnsByTable(
	ctx context.Context,
	databaseName string,
) (map[string][]*dtos.SelectForeignKeyColumnsDto, error) {
	data, err := m.Child.SelectForeignKeyColumns(ctx, databaseName, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error when selecting foreign key columns")
	}

	return fetchAllColumns(ctx, m, data)
}

func (m *AbstractSchemaManager) IntrospectTable(ctx context.Context, name string) (*assets.Table, error) {
	columns, err := m.ListTableColumns(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "error when listing table columns")
	}

	if len(columns) == 0 {
		return nil, errors.Errorf("table %s not found", name)
	}

	indexes, err := m.ListTableIndexes(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "error when listing indexes")
	}

	tableOptions, err := m.getTableOptions(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting table options")
	}

	foreignKeys, err := m.ListTableForeignKeys(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "error when listing foreign keys")
	}

	uniq := make([]*assets.UniqueConstraint, 0)
	opts := tableOptions.ToArray()

	return assets.NewTable(name, maps.Values(columns), maps.Values(indexes), uniq, foreignKeys, opts), nil
}

func (m *AbstractSchemaManager) ListViews(ctx context.Context) ([]*assets.View, error) {
	database, err := m.getDatabase(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting database")
	}

	items, err := m.Connection.FetchAllAssociative(ctx, m.Platform.GetListViewsSQL(database))
	if err != nil {
		return nil, errors.Wrap(err, "error when listing views")
	}

	result := make([]*assets.View, 0)

	for _, item := range items {
		result = append(result, m.Child.GetPortableViewDefinition(item))
	}

	return result, nil
}

func (m *AbstractSchemaManager) ListTableForeignKeys(ctx context.Context, table string) (
	[]*assets.ForeignKeyConstraint,
	error,
) {
	name := m.normalizeName(table)

	database, err := m.getDatabase(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting database")
	}

	columns, err := m.Child.SelectForeignKeyColumns(ctx, database, &name)
	if err != nil {
		return nil, errors.Wrap(err, "error when selecting foreign key columns")
	}

	return m.getPortableTableForeignKeysList(columns), nil
}

func (m *AbstractSchemaManager) getTableOptions(
	ctx context.Context,
	table string,
) (*dtos.FetchTableOptionsByTableDto, error) {
	normalizedName := m.normalizeName(table)

	database, err := m.getDatabase(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting database")
	}

	opts, err := m.Child.FetchTableOptionsByTable(ctx, database, &normalizedName)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching table options by table")
	}

	return opts[normalizedName], nil
}

type getPortableTableIndexesListOptionsSubDto struct {
	lengths []*int
	where   string
}

func (g *getPortableTableIndexesListOptionsSubDto) asMap() map[string]any {
	res := map[string]any{}

	if len(g.lengths) > 0 {
		res["lengths"] = g.lengths
	}

	if g.where != "" {
		res["where"] = g.where
	}

	return res
}

type getPortableTableIndexesListDto struct {
	name    string
	columns []string
	unique  bool
	primary bool
	flags   []string
	options *getPortableTableIndexesListOptionsSubDto
}

func (g *getPortableTableIndexesListDto) addColumn(s string) {
	g.columns = append(g.columns, s)
}

func (g *getPortableTableIndexesListDto) addOptionsLength(length *int) {
	g.options.lengths = append(g.options.lengths, length)
}

func (m *AbstractSchemaManager) GetPortableTableIndexesList(
	tableIndexes []*dtos.PortableTableIndexesDto,
	tableName string,
) map[string]*assets.Index {
	result := make(map[string]*getPortableTableIndexesListDto)

	for _, tableIndex := range tableIndexes {

		indexName := tableIndex.KeyName
		keyName := indexName
		if tableIndex.Primary {
			keyName = "primary"
		}

		keyName = strings.ToLower(keyName)

		if _, ok := result[keyName]; !ok {
			options := &getPortableTableIndexesListOptionsSubDto{lengths: make([]*int, 0)}
			if tableIndex.Where != nil {
				options.where = *tableIndex.Where
			}

			result[keyName] = &getPortableTableIndexesListDto{
				name:    indexName,
				columns: make([]string, 0),
				unique:  !tableIndex.NonUnique,
				primary: tableIndex.Primary,
				flags:   make([]string, 0),
				options: options,
			}
		}

		result[keyName].addColumn(tableIndex.ColumnName)
		//result[keyName].addOptionsLength(nil) // tableIndex.Length
	}

	indexes := make(map[string]*assets.Index)

	for indexKey, data := range result {
		indexes[indexKey] = assets.NewIndex(
			data.name,
			data.columns,
			data.unique,
			data.primary,
			data.flags,
			data.options.asMap(),
		)
	}

	return indexes
}

func (m *AbstractSchemaManager) getPortableTableForeignKeysList(tableForeignKeys []*dtos.SelectForeignKeyColumnsDto) []*assets.ForeignKeyConstraint {
	list := make([]*assets.ForeignKeyConstraint, 0)

	for _, value := range tableForeignKeys {
		list = append(list, m.Child.GetPortableTableForeignKeyDefinition(value))
	}

	return list
}

func (m *AbstractSchemaManager) IntrospectSchema(ctx context.Context) (*assets.Schema, error) {
	s := m.Child

	var err error

	schemaNames := make([]string, 0)

	if m.Platform.SupportsSchemas() {
		schemaNames, err = s.ListSchemaNames(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error when listing schema names")
		}
	}

	sequences := make([]*assets.Sequence, 0)

	if m.Platform.SupportsSequences() {
		sequences, err = s.ListSequences(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error when listing sequences")
		}
	}

	tables, err := s.ListTables(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when listing tables")
	}

	// Remove schema_migrations
	for i, table := range tables {
		if slices.Contains([]string{"schema_migrations", "goose_db_version"}, table.GetName()) {
			tables = slices.Delete(tables, i, i+1)
			break
		}
	}

	schemaConfig, err := s.CreateSchemaConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when creating schema config")
	}

	return assets.NewSchema(tables, sequences, schemaConfig, schemaNames), nil
}

func (m *AbstractSchemaManager) CreateSchemaConfig() *dtos.SchemaConfig {
	schemaConfig := dtos.NewSchemaConfig()
	schemaConfig.SetMaxIdentifierLength(m.Platform.GetMaxIdentifierLength())

	return schemaConfig
}

func (m *AbstractSchemaManager) getDatabase(ctx context.Context) (string, error) {
	return m.Connection.GetDatabase(ctx)
}

func (m *AbstractSchemaManager) CreateComparator() *diff_calc.Comparator {
	return diff_calc.NewComparator(m.Platform)
}

func fetchAllColumns[T dtos.GetPortableTableDefinitionInputDto](
	ctx context.Context,
	schemaManager *AbstractSchemaManager,
	typedData []T,
) (map[string][]T, error) {
	data := make(map[string][]T)

	for _, row := range typedData {
		tableName, err := schemaManager.Child.GetPortableTableDefinition(ctx, row)
		if err != nil {
			return nil, errors.Wrap(err, "error when getting portable table definition")
		}

		if _, ok := data[tableName]; !ok {
			data[tableName] = make([]T, 0)
		}

		data[tableName] = append(data[tableName], row)
	}

	return data, nil
}

func (m *AbstractSchemaManager) GetPortableTableColumnList(
	table string,
	database string,
	tableColumns []*dtos.SelectTableColumnsDto,
) map[string]*assets.Column {
	list := make(map[string]*assets.Column)

	for _, tableColumn := range tableColumns {
		column := m.Child.GetPortableTableColumnDefinition(tableColumn)

		name := strings.ToLower(column.GetQuotedName(m.Platform))
		list[name] = column
	}

	return list
}

func (m *AbstractSchemaManager) AlterSchemaSqlList(schemaDiff *diff_dtos.SchemaDiff) []string {
	mappedRows := make([]string, 0)

	for _, sql := range m.Platform.GetAlterSchemaSQL(schemaDiff) {
		mappedRows = append(mappedRows, fmt.Sprintf("%s;", sql))
	}

	return mappedRows
}

func (m *AbstractSchemaManager) AlterSchema(schemaDiff *diff_dtos.SchemaDiff) string {
	comment := "-- THIS FILE WAS GENERATED BY GORMITE, EDIT IT IF YOU WANT <3"

	mappedRows := m.AlterSchemaSqlList(schemaDiff)

	mappedRows = append([]string{comment, ""}, mappedRows...)

	return strings.Join(mappedRows, "\n")
}
