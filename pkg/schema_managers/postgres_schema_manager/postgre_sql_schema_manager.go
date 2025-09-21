package postgres_schema_manager

import (
	"context"
	"fmt"
	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/dtos"
	"github.com/KoNekoD/gormite/pkg/platforms"
	"github.com/KoNekoD/gormite/pkg/schema_managers/abstract_schema_managers"
	"github.com/KoNekoD/gormite/pkg/types"
	"github.com/pkg/errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type PostgreSQLSchemaManager struct {
	*abstract_schema_managers.AbstractSchemaManager
	currentSchema *string
}

func NewPostgreSQLSchemaManager(
	connection *platforms.Connection,
	platform platforms.AbstractPlatformInterface,
) *PostgreSQLSchemaManager {
	v := &PostgreSQLSchemaManager{
		AbstractSchemaManager: abstract_schema_managers.NewAbstractSchemaManager(
			connection,
			platform,
		),
	}

	v.Child = v

	return v
}

type ListSchemaNamesDto struct {
	SchemaName string `db:"schema_name"`
}

func (m *PostgreSQLSchemaManager) ListSchemaNames(ctx context.Context) ([]string, error) {
	items := make([]ListSchemaNamesDto, 0)

	sql := `
SELECT schema_name
FROM   information_schema.schemata
WHERE  schema_name NOT LIKE 'pg\_%'
AND    schema_name != 'information_schema'
`

	err := platforms.FetchScan(ctx, m.Connection, sql, &items)

	if err != nil {
		return nil, errors.Wrap(err, "error when listing schema names")
	}

	result := make([]string, len(items))
	for i, item := range items {
		result[i] = item.SchemaName
	}

	return result, nil
}

func (m *PostgreSQLSchemaManager) CreateSchemaConfig(ctx context.Context) (*dtos.SchemaConfig, error) {
	config := m.AbstractSchemaManager.CreateSchemaConfig()

	schemaName, err := m.getCurrentSchema(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting current schema")
	}

	config.SetName(schemaName)

	return config, nil
}

func (m *PostgreSQLSchemaManager) getCurrentSchema(ctx context.Context) (*string, error) {
	if m.currentSchema == nil {
		var err error
		m.currentSchema, err = m.determineCurrentSchema(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error when getting current schema")
		}
	}

	return m.currentSchema, nil
}

type determineCurrentSchemaDto struct {
	SchemaName string `db:"schema_name"`
}

func (m *PostgreSQLSchemaManager) determineCurrentSchema(ctx context.Context) (*string, error) {
	var dto determineCurrentSchemaDto
	sql := `SELECT current_schema() AS schema_name`
	err := platforms.FetchScan(ctx, m.Connection, sql, &dto)
	if err != nil {
		return nil, errors.Wrap(err, "error when determining current schema")
	}

	if dto.SchemaName == "" {
		return nil, nil
	}

	return &dto.SchemaName, nil
}

func (m *PostgreSQLSchemaManager) GetPortableTableForeignKeyDefinition(
	tableForeignKey *dtos.SelectForeignKeyColumnsDto,
) *assets.ForeignKeyConstraint {
	var onUpdate, onDelete *string

	onUpdateRegex := regexp.MustCompile(`ON UPDATE ([a-zA-Z0-9]+( (NULL|ACTION|DEFAULT))?)`)
	onDeleteRegex := regexp.MustCompile(`ON DELETE ([a-zA-Z0-9]+( (NULL|ACTION|DEFAULT))?)`)

	// Check for ON UPDATE condition
	if match := onUpdateRegex.FindStringSubmatch(tableForeignKey.Condef); len(match) > 0 {
		onUpdate = &match[1]
	}

	// Check for ON DELETE condition
	if match := onDeleteRegex.FindStringSubmatch(tableForeignKey.Condef); len(match) > 0 {
		onDelete = &match[1]
	}

	foreignKeyRegex := regexp.MustCompile(`FOREIGN KEY \((.+)\) REFERENCES (.+)\((.+)\)`)

	// Parse the FOREIGN KEY constraint
	if match := foreignKeyRegex.FindStringSubmatch(tableForeignKey.Condef); len(match) == 4 {
		localColumns := strings.Split(match[1], ",")
		for i := range localColumns {
			localColumns[i] = strings.TrimSpace(localColumns[i])
		}

		foreignColumns := strings.Split(match[3], ",")
		for i := range foreignColumns {
			foreignColumns[i] = strings.TrimSpace(foreignColumns[i])
		}

		foreignTable := match[2]

		return assets.NewForeignKeyConstraint(
			tableForeignKey.Conname,
			localColumns,
			foreignTable,
			foreignColumns,
			map[string]any{
				"onUpdate": onUpdate,
				"onDelete": onDelete,
			},
		)
	}

	panic(fmt.Errorf("invalid foreign key definition"))
}

func (m *PostgreSQLSchemaManager) GetPortableDatabaseDefinition(row map[string]any) string {
	return row["datname"].(string)
}

func (m *PostgreSQLSchemaManager) GetPortableSequenceDefinition(sequence *dtos.ListSequencesDto) *assets.Sequence {
	sequenceName := ""
	if sequence.Schemaname != "public" {
		sequenceName = sequence.Schemaname + "." + sequence.Relname
	} else {
		sequenceName = sequence.Relname
	}

	return assets.NewSequence(
		sequenceName,
		assets.WithAllocationSize(sequence.GetIncrementBy()),
		assets.WithInitialValue(sequence.GetMinValue()),
	)
}

func (m *PostgreSQLSchemaManager) GetPortableTableColumnDefinition(tableColumn *dtos.SelectTableColumnsDto) *assets.Column {
	var length *int

	if slices.Contains([]string{"varchar", "bpchar"}, tableColumn.Type) {
		matches := regexp.MustCompile(`\((\d*)\)`).FindStringSubmatch(tableColumn.CompleteType)
		if len(matches) == 2 {
			lenInt, _ := strconv.Atoi(matches[1])
			length = &lenInt
		}
	}

	autoincrement := tableColumn.Attidentity == 'd'

	matches := make([]string, 0)

	_ = tableColumn.Default
	_ = tableColumn.CompleteType

	if tableColumn.Default != nil {
		if matches = regexp.MustCompile(`^['(](.*)[')]::`).FindStringSubmatch(*tableColumn.Default); len(matches) == 2 {
			tableColumn.Default = &matches[1]
		} else if matches = regexp.MustCompile(`^NULL::`).FindStringSubmatch(*tableColumn.Default); len(matches) == 2 {
			tableColumn.Default = nil
		}
	}

	if length != nil && *length <= 0 {
		length = nil
	}

	fixed := false

	var precision *int
	var scale *int
	var jsonb *bool

	dbType := strings.ToLower(tableColumn.Type)

	if tableColumn.DomainType != nil && *tableColumn.DomainType != "" && !m.Platform.HasDoctrineTypeMappingFor(dbType) {
		dbType = strings.ToLower(*tableColumn.DomainType)
		tableColumn.CompleteType = *tableColumn.DomainCompleteType
	}

	typeMapping := m.Platform.GetDoctrineTypeMapping(dbType)

	switch dbType {
	case "smallint", "int2", "int", "int4", "integer", "bigint", "int8":
		length = nil

	case "bool", "boolean":
		length = nil
	case "json", "text", "_varchar", "varchar":
		tableColumn.Default = m.parseDefaultExpression(tableColumn.Default)
	case "char", "bpchar":
		fixed = true
	case "float", "float4", "float8", "double", "double precision", "real", "decimal", "money", "numeric":
		re := regexp.MustCompile(`([A-Za-z]+\(([0-9]+),([0-9]+)\))`)
		matches := re.FindStringSubmatch(tableColumn.CompleteType)

		if len(matches) > 2 {
			precisionInt, _ := strconv.Atoi(matches[2])
			precision = &precisionInt
			scaleInt, _ := strconv.Atoi(matches[3])
			scale = &scaleInt
			length = nil
		}
	case "year":
		length = nil
	case "jsonb":
		jsonbTmp := true
		jsonb = &jsonbTmp
	}

	if tableColumn.Default != nil {
		re := regexp.MustCompile(`'([^']+)'::`)
		if matches = re.FindStringSubmatch(*tableColumn.Default); len(matches) == 2 {
			tableColumn.Default = &matches[1]
		}
	}

	options := make([]assets.ColumnOption, 0)
	options = append(options, assets.WithColumnLength(length))

	if tableColumn.IsNotnull {
		options = append(options, assets.WithColumnNotNull())
	}

	if tableColumn.Default != nil {
		options = append(options, assets.WithColumnDefault(*tableColumn.Default))
	}

	options = append(options, assets.WithColumnPrecision(precision))

	options = append(options, assets.WithColumnScale(scale))

	if fixed {
		options = append(options, assets.WithColumnFixed())
	}

	if autoincrement {
		options = append(options, assets.WithColumnAutoIncrement())
	}

	if tableColumn.Comment != nil {
		options = append(options, assets.WithColumnComment(*tableColumn.Comment))
	}

	column := assets.NewColumn(tableColumn.Field, types.GetType(typeMapping), options...)

	if tableColumn.Collation != nil {
		column.SetPlatformOption("collation", *tableColumn.Collation)
	}

	if _, ok := column.GetColumnType().(*types.JsonType); ok {
		column.SetPlatformOption("jsonb", jsonb)
	}

	return column
}

func (m *PostgreSQLSchemaManager) GetPortableViewDefinition(view map[string]any) *assets.View {
	return assets.NewView(
		view["schemaname"].(string)+"."+view["viewname"].(string),
		view["definition"].(string),
	)
}

func (m *PostgreSQLSchemaManager) GetPortableTableIndexesList(
	ctx context.Context,
	tableIndexes []*dtos.SelectIndexColumnsDto,
	tableName string,
) (map[string]*assets.Index, error) {
	buffer := make([]*dtos.PortableTableIndexesDto, 0)

	for _, row := range tableIndexes {
		colNumbers := strings.Split(row.Indkey, " ")

		columnNameSql := fmt.Sprintf(
			"SELECT attnum, attname FROM pg_attribute WHERE attrelid=%s AND attnum IN (%s) ORDER BY attnum ASC",
			*row.Indrelid,
			strings.Join(colNumbers, " ,"),
		)

		indexColumns := make([]dtos.GetColNameDto, 0)
		err := platforms.FetchScan(ctx, m.Connection, columnNameSql, &indexColumns)
		if err != nil {
			return nil, errors.Wrap(err, "error when fetching column name")
		}

		for _, colNum := range colNumbers {
			for _, colRow := range indexColumns {
				if colNum != colRow.Attnum {
					continue
				}

				buffer = append(
					buffer, &dtos.PortableTableIndexesDto{
						KeyName:    row.RelName,
						ColumnName: strings.TrimSpace(colRow.Attname),
						NonUnique:  !row.IndisUnique,
						Primary:    row.IndisPrimary,
						Where:      row.Where,
					},
				)
			}
		}
	}

	return m.AbstractSchemaManager.GetPortableTableIndexesList(buffer, tableName), nil
}

func (m *PostgreSQLSchemaManager) GetPortableTableDefinition(
	ctx context.Context,
	table dtos.GetPortableTableDefinitionInputDto,
) (string, error) {
	currentSchema, err := m.getCurrentSchema(ctx)
	if err != nil {
		return "", errors.Wrap(err, "error when getting current schema")
	}

	if table.GetSchemaName() == *currentSchema {
		return table.GetTableName(), nil
	}

	return table.GetSchemaName() + "." + table.GetTableName(), nil
}

func (m *PostgreSQLSchemaManager) SelectTableColumns(
	ctx context.Context,
	databaseName string,
	tableName *string,
) ([]*dtos.SelectTableColumnsDto, error) {
	sql := "SELECT "

	if tableName == nil {
		sql += "c.relname AS table_name, n.nspname AS schema_name,"
	}

	sql += fmt.Sprintf(
		`
	           a.attnum,
	           quote_ident(a.attname) AS field,
	           t.typname AS type,
	           format_type(a.atttypid, a.atttypmod) AS complete_type,
	           (SELECT tc.collcollate FROM pg_catalog.pg_collation tc WHERE tc.oid = a.attcollation) AS collation,
	           (SELECT t1.typname FROM pg_catalog.pg_type t1 WHERE t1.oid = t.typbasetype) AS domain_type,
	           (SELECT format_type(t2.typbasetype, t2.typtypmod) FROM
	             pg_catalog.pg_type t2 WHERE t2.typtype = 'd' AND t2.oid = a.atttypid) AS domain_complete_type,
	           a.attnotnull AS isnotnull,
	           a.attidentity,
	           (SELECT 't'
	            FROM pg_index
	            WHERE c.oid = pg_index.indrelid
	               AND pg_index.indkey[0] = a.attnum
	               AND pg_index.indisprimary = 't'
	           ) AS pri,
	           (%s) AS default,
	           (SELECT pg_description.description
	               FROM pg_description WHERE pg_description.objoid = c.oid AND a.attnum = pg_description.objsubid
	           ) AS comment
	           FROM pg_attribute a
	               INNER JOIN pg_class c
	                   ON c.oid = a.attrelid
	               INNER JOIN pg_type t
	                   ON t.oid = a.atttypid
	               INNER JOIN pg_namespace n
	                   ON n.oid = c.relnamespace
	               LEFT JOIN pg_depend d
	                   ON d.objid = c.oid
	                       AND d.deptype = 'e'
	                       AND d.classid = (SELECT oid FROM pg_class WHERE relname = 'pg_class')
	           `, m.Platform.GetDefaultColumnValueSQLSnippet(),
	)

	conditions := make([]string, 0)
	conditions = append(conditions, "a.attnum > 0")
	conditions = append(conditions, "c.relkind = 'r'")
	conditions = append(conditions, "d.refobjid IS NULL")
	conditions = append(conditions, m.buildQueryConditions(tableName)...)

	sql += " WHERE " + strings.Join(conditions, " AND ") + " ORDER BY a.attnum"

	items := make([]dtos.SelectTableColumnsDto, 0)

	err := platforms.FetchScan(ctx, m.Connection, sql, &items)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching table columns")
	}

	columns := make([]*dtos.SelectTableColumnsDto, 0)
	for _, item := range items {
		columns = append(columns, &item)
	}

	return columns, nil
}

func (m *PostgreSQLSchemaManager) SelectIndexColumns(
	ctx context.Context,
	databaseName string,
	tableName *string,
) ([]*dtos.SelectIndexColumnsDto, error) {
	sql := "SELECT"

	if tableName == nil {
		sql += " tc.relname AS table_name, tn.nspname AS schema_name,"
	}

	sql += `
	quote_ident(ic.relname) AS relname,
		i.indisunique,
		i.indisprimary,
		i.indkey,
		i.indrelid,
		pg_get_expr(indpred, indrelid) AS "where"
	FROM pg_index i
	JOIN pg_class AS tc ON tc.oid = i.indrelid
	JOIN pg_namespace tn ON tn.oid = tc.relnamespace
	JOIN pg_class AS ic ON ic.oid = i.indexrelid
	WHERE ic.oid IN (
		SELECT indexrelid
	FROM pg_index i, pg_class c, pg_namespace n
	`

	conditions := make([]string, 0)
	conditions = append(conditions, "c.oid = i.indrelid")
	conditions = append(conditions, "c.relnamespace = n.oid")
	conditions = append(conditions, m.buildQueryConditions(tableName)...)

	sql += " WHERE " + strings.Join(conditions, " AND ") + ")"

	items := make([]dtos.SelectIndexColumnsDto, 0)
	err := platforms.FetchScan(ctx, m.Connection, sql, &items)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching index columns")
	}

	columns := make([]*dtos.SelectIndexColumnsDto, 0)
	for _, item := range items {
		columns = append(columns, &item)
	}

	return columns, nil
}

func (m *PostgreSQLSchemaManager) SelectTableNames(
	ctx context.Context,
	databaseName string,
) ([]*dtos.SelectTableNamesDto, error) {
	sql := `
SELECT quote_ident(table_name) AS table_name,
	table_schema AS schema_name
FROM information_schema.tables
WHERE table_catalog = '` + databaseName + `'
AND table_schema NOT LIKE 'pg\_%'
AND table_schema != 'information_schema'
AND table_name != 'geometry_columns'
AND table_name != 'spatial_ref_sys'
AND table_type = 'BASE TABLE'
	`

	items := make([]dtos.SelectTableNamesDto, 0)
	err := platforms.FetchScan(ctx, m.Connection, sql, &items)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching table names")
	}

	names := make([]*dtos.SelectTableNamesDto, 0)
	for _, item := range items {
		names = append(names, &item)
	}

	return names, nil
}

func (m *PostgreSQLSchemaManager) SelectForeignKeyColumns(
	ctx context.Context,
	databaseName string,
	tableName *string,
) ([]*dtos.SelectForeignKeyColumnsDto, error) {
	sql := "SELECT"

	if tableName == nil {
		sql += " tc.relname AS table_name, tn.nspname AS schema_name,"
	}

	sql += `
	quote_ident(r.conname) as conname,
		pg_get_constraintdef(r.oid, true) as condef
	FROM pg_constraint r
	JOIN pg_class AS tc ON tc.oid = r.conrelid
	JOIN pg_namespace tn ON tn.oid = tc.relnamespace
	WHERE r.conrelid IN
	(
		SELECT c.oid
	FROM pg_class c, pg_namespace n
	`

	conditions := make([]string, 0)
	conditions = append(conditions, "n.oid = c.relnamespace")
	conditions = append(conditions, m.buildQueryConditions(tableName)...)

	sql += " WHERE " + strings.Join(conditions, " AND ") + ") AND r.contype = 'f'"

	items := make([]dtos.SelectForeignKeyColumnsDto, 0)
	err := platforms.FetchScan(ctx, m.Connection, sql, &items)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching foreign key columns")
	}

	columns := make([]*dtos.SelectForeignKeyColumnsDto, 0)
	for _, item := range items {
		columns = append(columns, &item)
	}

	return columns, nil
}

func (m *PostgreSQLSchemaManager) FetchTableOptionsByTable(
	ctx context.Context,
	databaseName string,
	tableName *string,
) (map[string]*dtos.FetchTableOptionsByTableDto, error) {
	sql := `
	SELECT c.relname,
		CASE c.relpersistence WHEN 'u' THEN true ELSE false END as unlogged,
		obj_description(c.oid, 'pg_class') AS comment
	FROM pg_class c
	INNER JOIN pg_namespace n
	ON n.oid = c.relnamespace
	`

	conditions := make([]string, 0)
	conditions = append(conditions, "c.relkind = 'r'")
	conditions = append(conditions, m.buildQueryConditions(tableName)...)

	sql += " WHERE " + strings.Join(conditions, " AND ")

	result := make(map[string]*dtos.FetchTableOptionsByTableDto)

	items := make([]dtos.FetchTableOptionsByTableDto, 0)
	err := platforms.FetchScan(ctx, m.Connection, sql, &items)
	if err != nil {
		return nil, errors.Wrap(err, "error when fetching table options by table")
	}

	for _, t := range items {
		if result[t.Relname] != nil {
			panic(fmt.Sprintf("duplicate table name: %s", t.Relname))
		}
		result[t.Relname] = &t
	}

	return result, nil
}

func (m *PostgreSQLSchemaManager) buildQueryConditions(tableName *string) []string {
	conditions := make([]string, 0)

	if tableName != nil {
		tableNameStr := *tableName
		if strings.Contains(*tableName, ".") {
			parts := strings.Split(*tableName, ".")
			schemaName := parts[0]
			tableNameStr = parts[1]
			conditions = append(conditions, "n.nspname = "+m.Platform.QuoteStringLiteral(schemaName))
		} else {
			conditions = append(conditions, "n.nspname = ANY(current_schemas(false))")
		}

		identifier := assets.NewIdentifier(tableNameStr)
		conditions = append(conditions, "c.relname = "+m.Platform.QuoteStringLiteral(identifier.GetName()))
	}

	conditions = append(conditions, "n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')")

	return conditions
}

func (m *PostgreSQLSchemaManager) parseDefaultExpression(defaultExpression *string) *string {
	if defaultExpression == nil {
		return nil
	}

	expr := strings.ReplaceAll(*defaultExpression, "''", "'")

	return &expr
}
