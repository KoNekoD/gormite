package platforms

import (
	"fmt"
	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/diff_dtos"
	"github.com/KoNekoD/gormite/pkg/enums"
	"github.com/KoNekoD/gormite/pkg/keywords"
	"github.com/KoNekoD/gormite/pkg/sql_builders"
	"github.com/KoNekoD/gormite/pkg/types"
	"github.com/KoNekoD/gormite/pkg/utils"
	"github.com/elliotchance/pie/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type AbstractPlatform struct {
	DoctrineTypeMapping map[string]enums.TypesType
	keywords            keywords.KeywordListInterface
	child               AbstractPlatformInterface
}

func NewAbstractPlatform(child AbstractPlatformInterface) *AbstractPlatform {
	return &AbstractPlatform{
		keywords: keywords.NewPostgreSQLKeywords(),
		child:    child,
	}
}

func (parent *AbstractPlatform) InitializeAllDoctrineTypeMappings() {
	a := parent.child
	a.InitializeDoctrineTypeMappings()

	for typeVarName := range types.GetTypesMap() {
		gotType := types.GetType(typeVarName)
		for _, dbType := range gotType.GetMappedDatabaseTypes(a) {
			dbType = strings.ToLower(dbType)
			parent.DoctrineTypeMapping[dbType] = typeVarName
		}
	}
}

func (parent *AbstractPlatform) ExtractLength(data map[string]any) *int {
	if lengthAny, ok := data[`length`]; ok {
		switch lengthValue := lengthAny.(type) {
		case int:
			return &lengthValue
		case *int:
			return lengthValue
		default:
			panic(fmt.Errorf("invalid length type %T", lengthAny))
		}
	}

	return nil
}

func (parent *AbstractPlatform) GetStringTypeDeclarationSQL(column map[string]any) string {
	a := parent.child
	length := a.ExtractLength(column)

	if v, ok := column[`fixed`]; ok && v == true {
		return a.GetCharTypeDeclarationSQLSnippet(length)
	}

	return a.GetVarcharTypeDeclarationSQLSnippet(length)
}
func (parent *AbstractPlatform) GetBinaryTypeDeclarationSQL(column map[string]any) string {
	a := parent.child
	length := a.ExtractLength(column)

	if _, ok := column[`fixed`]; ok {
		return a.GetBinaryTypeDeclarationSQLSnippet(length)
	}

	return a.GetVarbinaryTypeDeclarationSQLSnippet(length)
}
func (parent *AbstractPlatform) GetGuidTypeDeclarationSQL(column map[string]any) string {
	a := parent.child
	column[`length`] = 36
	column[`fixed`] = true

	return a.GetStringTypeDeclarationSQL(column)
}
func (parent *AbstractPlatform) GetJsonTypeDeclarationSQL(column map[string]any) string {
	a := parent.child
	return a.GetClobTypeDeclarationSQL(column)
}
func (parent *AbstractPlatform) GetCharTypeDeclarationSQLSnippet(length *int) string {
	sql := `CHAR`

	if length != nil {
		sql += fmt.Sprintf(`(%d)`, *length)
	}

	return sql
}
func (parent *AbstractPlatform) GetVarcharTypeDeclarationSQLSnippet(length *int) string {
	if length == nil {
		panic("Column length must be specified for VARCHAR columns")
	}

	return fmt.Sprintf(`VARCHAR(%d)`, *length)
}
func (parent *AbstractPlatform) GetBinaryTypeDeclarationSQLSnippet(length *int) string {
	sql := `BINARY`

	if length != nil {
		sql += fmt.Sprintf(`(%d)`, *length)
	}

	return sql
}
func (parent *AbstractPlatform) GetVarbinaryTypeDeclarationSQLSnippet(length *int) string {
	if length == nil {
		panic("Column length must be specified for VARCHAR columns")
	}

	return fmt.Sprintf(`VARBINARY(%d)`, *length)
}

func (parent *AbstractPlatform) RegisterDoctrineTypeMapping(
	dbType string,
	doctrineType string,
) {
	a := parent.child
	if parent.DoctrineTypeMapping == nil {
		a.InitializeAllDoctrineTypeMappings()
	}

	if !types.HasType(enums.TypesType(doctrineType)) {
		panic("TypeNotFound " + doctrineType)
	}

	dbType = strings.ToLower(dbType)
	parent.DoctrineTypeMapping[dbType] = enums.TypesType(doctrineType)
}
func (parent *AbstractPlatform) GetDoctrineTypeMapping(dbType string) enums.TypesType {
	a := parent.child
	if parent.DoctrineTypeMapping == nil {
		a.InitializeAllDoctrineTypeMappings()
	}

	dbType = strings.ToLower(dbType)

	v, hasDoctrineTypeMapping := parent.DoctrineTypeMapping[dbType]
	if !hasDoctrineTypeMapping {
		panic(
			fmt.Errorf(
				`unknown database type "%s" requested, %T may not support it`,
				dbType,
				a,
			),
		)
	}

	return v
}
func (parent *AbstractPlatform) HasDoctrineTypeMappingFor(dbType string) bool {
	a := parent.child
	if parent.DoctrineTypeMapping == nil {
		a.InitializeAllDoctrineTypeMappings()
	}

	dbType = strings.ToLower(dbType)

	_, ok := parent.DoctrineTypeMapping[dbType]

	return ok
}
func (parent *AbstractPlatform) GetRegexpExpression() string {
	panic("Not supported")
}
func (parent *AbstractPlatform) GetLengthExpression(string string) string {
	return `LENGTH(` + string + `)`
}
func (parent *AbstractPlatform) GetModExpression(
	dividend string,
	divisor string,
) string {
	return `MOD(` + dividend + `, ` + divisor + `)`
}
func (parent *AbstractPlatform) GetTrimExpression(
	str string,
	mode enums.TrimMode,
	char *string,
) string {
	tokens := make([]string, 0)

	switch mode {
	case enums.TrimModeUnspecified:
		break

	case enums.TrimModeLeading:
		tokens = append(tokens, `LEADING`)
		break

	case enums.TrimModeTrailing:
		tokens = append(tokens, `TRAILING`)
		break

	case enums.TrimModeBoth:
		tokens = append(tokens, `BOTH`)
		break
	}

	if char != nil {
		tokens = append(tokens, *char)
	}

	if len(tokens) > 0 {
		tokens = append(tokens, `FROM`)
	}

	tokens = append(tokens, str)

	return fmt.Sprintf(`TRIM(%s)`, strings.Join(tokens, ` `))
}

func (parent *AbstractPlatform) GetSubstringExpression(
	string string,
	start string,
	length *string,
) string {
	if length == nil {
		return fmt.Sprintf(`SUBSTRING(%s FROM %s)`, string, start)
	}

	return fmt.Sprintf(`SUBSTRING(%s FROM %s FOR %s)`, string, start, *length)
}
func (parent *AbstractPlatform) GetConcatExpression(string ...string) string {
	return strings.Join(string, ` || `)
}

func (parent *AbstractPlatform) GetDateAddSecondsExpression(
	date string,
	seconds string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`+`,
		seconds,
		enums.DateIntervalUnitSecond,
	)
}
func (parent *AbstractPlatform) GetDateSubSecondsExpression(
	date string,
	seconds string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`-`,
		seconds,
		enums.DateIntervalUnitSecond,
	)
}
func (parent *AbstractPlatform) GetDateAddMinutesExpression(
	date string,
	minutes string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`+`,
		minutes,
		enums.DateIntervalUnitMinute,
	)
}
func (parent *AbstractPlatform) GetDateSubMinutesExpression(
	date string,
	minutes string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`-`,
		minutes,
		enums.DateIntervalUnitMinute,
	)
}
func (parent *AbstractPlatform) GetDateAddHourExpression(
	date string,
	hours string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`+`,
		hours,
		enums.DateIntervalUnitHour,
	)
}
func (parent *AbstractPlatform) GetDateSubHourExpression(
	date string,
	hours string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`-`,
		hours,
		enums.DateIntervalUnitHour,
	)
}
func (parent *AbstractPlatform) GetDateAddDaysExpression(
	date string,
	days string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`+`,
		days,
		enums.DateIntervalUnitDay,
	)
}
func (parent *AbstractPlatform) GetDateSubDaysExpression(
	date string,
	days string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`-`,
		days,
		enums.DateIntervalUnitDay,
	)
}
func (parent *AbstractPlatform) GetDateAddWeeksExpression(
	date string,
	weeks string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`+`,
		weeks,
		enums.DateIntervalUnitWeek,
	)
}
func (parent *AbstractPlatform) GetDateSubWeeksExpression(
	date string,
	weeks string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`-`,
		weeks,
		enums.DateIntervalUnitWeek,
	)
}
func (parent *AbstractPlatform) GetDateAddMonthExpression(
	date string,
	months string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`+`,
		months,
		enums.DateIntervalUnitMonth,
	)
}
func (parent *AbstractPlatform) GetDateSubMonthExpression(
	date string,
	months string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`-`,
		months,
		enums.DateIntervalUnitMonth,
	)
}
func (parent *AbstractPlatform) GetDateAddQuartersExpression(
	date string,
	quarters string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`+`,
		quarters,
		enums.DateIntervalUnitQuarter,
	)
}
func (parent *AbstractPlatform) GetDateSubQuartersExpression(
	date string,
	quarters string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`-`,
		quarters,
		enums.DateIntervalUnitQuarter,
	)
}
func (parent *AbstractPlatform) GetDateAddYearsExpression(
	date string,
	years string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`+`,
		years,
		enums.DateIntervalUnitYear,
	)
}
func (parent *AbstractPlatform) GetDateSubYearsExpression(
	date string,
	years string,
) string {
	a := parent.child
	return a.GetDateArithmeticIntervalExpression(
		date,
		`-`,
		years,
		enums.DateIntervalUnitYear,
	)
}

func (parent *AbstractPlatform) MultiplyInterval(
	interval string,
	multiplier int,
) string {
	return fmt.Sprintf(`(%s * %d)`, interval, multiplier)
}
func (parent *AbstractPlatform) GetBitAndComparisonExpression(
	value1 string,
	value2 string,
) string {
	return `(` + value1 + ` & ` + value2 + `)`
}
func (parent *AbstractPlatform) GetBitOrComparisonExpression(
	value1 string,
	value2 string,
) string {
	return `(` + value1 + ` | ` + value2 + `)`
}

func (parent *AbstractPlatform) GetDropTableSQL(table string) string {
	return `DROP TABLE ` + table
}
func (parent *AbstractPlatform) GetDropTemporaryTableSQL(table string) string {
	a := parent.child
	return a.GetDropTableSQL(table)
}
func (parent *AbstractPlatform) GetDropIndexSQL(
	name string,
	table string,
) string {
	return `DROP INDEX ` + name
}

func (parent *AbstractPlatform) GetDropConstraintSQL(
	name string,
	table string,
) string {
	return `ALTER TABLE ` + table + ` DROP CONSTRAINT ` + name
}
func (parent *AbstractPlatform) GetDropForeignKeySQL(
	foreignKey string,
	table string,
) string {
	return `ALTER TABLE ` + table + ` DROP FOREIGN KEY ` + foreignKey
}
func (parent *AbstractPlatform) GetDropUniqueConstraintSQL(
	name string,
	tableName string,
) string {
	a := parent.child
	return a.GetDropConstraintSQL(name, tableName)
}
func (parent *AbstractPlatform) GetCreateTableSQL(table *assets.Table) []string {
	a := parent.child
	return a.BuildCreateTableSQL(table, true)
}
func (parent *AbstractPlatform) CreateSelectSQLBuilder() sql_builders.SelectSQLBuilder {
	a := parent.child
	return sql_builders.NewDefaultSelectSQLBuilder(
		a,
		`FOR UPDATE`,
		`SKIP LOCKED`,
	)
}
func (parent *AbstractPlatform) CreateUnionSQLBuilder() sql_builders.UnionSQLBuilder {
	a := parent.child
	return sql_builders.NewDefaultUnionSQLBuilder(a)
}
func (parent *AbstractPlatform) GetCreateTableWithoutForeignKeysSQL(table *assets.Table) []string {
	a := parent.child
	return a.BuildCreateTableSQL(table, false)
}
func (parent *AbstractPlatform) BuildCreateTableSQL(table *assets.Table, createForeignKeys bool) []string {
	a := parent.child
	if table.LenColumns() == 0 {
		panic("NoColumnsSpecifiedForTable")
	}

	uniqueConstraints := make(map[string]*assets.UniqueConstraint)
	indexes := make(map[string]*assets.Index)
	primary := make([]string, 0)
	var primaryIndex *assets.Index
	foreignKeys := make([]*assets.ForeignKeyConstraint, 0)

	for _, index := range table.GetIndexes() {
		if !index.IsPrimary() {
			indexes[index.GetQuotedName(a)] = index

			continue
		}

		if primaryIndex != nil {
			panic("Can be only one primary index")
		}

		primary = index.GetQuotedColumns(a)
		primaryIndex = index
	}
	for _, uniqueConstraint := range table.GetUniqueConstraints() {
		uniqueConstraints[uniqueConstraint.GetQuotedName(a)] = uniqueConstraint
	}
	if createForeignKeys {
		for _, fkConstraint := range table.GetForeignKeys() {
			foreignKeys = append(foreignKeys, fkConstraint)
		}
	}

	tableName := table.GetQuotedName(a)
	options := table.GetOptions()

	options[`indexes`] = indexes
	options[`primary`] = primary
	options[`primary_index`] = primaryIndex
	options[`uniqueConstraints`] = uniqueConstraints
	options[`foreignKeys`] = foreignKeys

	columns := make([]map[string]any, 0)
	for column := range table.GetColumns() {
		columnData := a.ColumnToArray(column)

		if slices.Contains(primary, column.GetName()) {
			columnData[`primary`] = true
		}

		columns = append(columns, columnData)
	}

	sql := a.GetCreateTableInnerSQL(tableName, columns, options)

	if a.SupportsCommentOnStatement() {
		if table.HasOption(`comment`) {
			if v := table.GetOption(`comment`).(*string); v != nil {
				sql = append(sql, a.GetCommentOnTableSQL(tableName, *v))
			}
		}

		for column := range table.GetColumns() {
			comment := column.GetComment()

			if comment == `` {
				continue
			}

			sql = append(sql, a.GetCommentOnColumnSQL(tableName, column.GetQuotedName(a), comment))
		}
	}

	return sql
}
func (parent *AbstractPlatform) GetCreateTablesSQL(tables []*assets.Table) []string {
	a := parent.child
	sql := make([]string, 0)

	for _, table := range tables {
		sql = append(sql, a.GetCreateTableWithoutForeignKeysSQL(table)...)
	}

	for _, table := range tables {
		for _, foreignKey := range table.GetForeignKeys() {
			sql = append(
				sql,
				a.GetCreateForeignKeySQL(foreignKey, table.GetQuotedName(a)),
			)
		}
	}

	return sql
}
func (parent *AbstractPlatform) GetDropTablesSQL(tables []*assets.Table) []string {
	a := parent.child
	sql := make([]string, 0)

	for _, table := range tables {
		for _, foreignKey := range table.GetForeignKeys() {
			sql = append(
				sql,
				a.GetDropForeignKeySQL(
					foreignKey.GetQuotedName(a),
					table.GetQuotedName(a),
				),
			)
		}
	}

	for _, table := range tables {
		sql = append(sql, a.GetDropTableSQL(table.GetQuotedName(a)))
	}

	return sql
}
func (parent *AbstractPlatform) GetCommentOnTableSQL(
	tableName string,
	comment string,
) string {
	a := parent.child
	tableNameId := assets.NewIdentifier(tableName)

	return fmt.Sprintf(
		`COMMENT ON TABLE %s IS %s`,
		tableNameId.GetQuotedName(a),
		a.QuoteStringLiteral(comment),
	)
}

func (parent *AbstractPlatform) GetCommentOnColumnSQL(
	tableName string,
	columnName string,
	comment string,
) string {
	a := parent.child
	tableNameId := assets.NewIdentifier(tableName)
	columnNameId := assets.NewIdentifier(columnName)

	return fmt.Sprintf(
		`COMMENT ON COLUMN %s.%s IS %s`,
		tableNameId.GetQuotedName(a),
		columnNameId.GetQuotedName(a),
		a.QuoteStringLiteral(comment),
	)
}

func (parent *AbstractPlatform) GetInlineColumnCommentSQL(comment string) string {
	a := parent.child
	if !a.SupportsInlineColumnComments() {
		panic("Not supported")
	}

	return `COMMENT ` + a.QuoteStringLiteral(comment)
}
func (parent *AbstractPlatform) GetCreateTableInnerSQL(
	name string,
	columns []map[string]any,
	options map[string]any,
) []string {
	a := parent.child
	columnListSql := a.GetColumnDeclarationListSQL(columns)

	if v, ok := options[`primary`]; ok {
		columnListSql += `, PRIMARY KEY(` + strings.Join(
			pie.Unique(v.([]string)),
			`, `,
		) + `)`
	}

	query := `CREATE TABLE ` + name + ` (` + columnListSql
	check := a.GetCheckDeclarationSQL(columns)

	if check != "" {
		query += `, ` + check
	}

	query += `)`

	sql := []string{query}

	if v, ok := options[`foreignKeys`]; ok {
		for _, definition := range v.([]*assets.ForeignKeyConstraint) {
			sql = append(sql, a.GetCreateForeignKeySQL(definition, name))
		}
	}

	if v, ok := options[`uniqueConstraints`]; ok {
		for _, definition := range v.(map[string]*assets.UniqueConstraint) {
			sql = append(sql, a.GetCreateUniqueConstraintSQL(definition, name))
		}
	}

	if v, ok := options[`indexes`]; ok {
		for _, definition := range v.(map[string]*assets.Index) {
			sql = append(sql, a.GetCreateIndexSQL(definition, name))
		}
	}

	return sql
}
func (parent *AbstractPlatform) GetCreateTemporaryTableSnippetSQL() string {
	return `CREATE TEMPORARY TABLE`
}
func (parent *AbstractPlatform) GetAlterSchemaSQL(diff *diff_dtos.SchemaDiff) []string {
	a := parent.child
	sql := make([]string, 0)

	if a.SupportsSchemas() {
		for _, schema := range diff.GetCreatedSchemas() {
			sql = append(sql, a.GetCreateSchemaSQL(schema))
		}
	}

	if a.SupportsSequences() {
		for _, sequence := range diff.GetAlteredSequences() {
			sql = append(sql, a.GetAlterSequenceSQL(sequence))
		}

		for _, sequence := range diff.GetDroppedSequences() {
			sql = append(sql, a.GetDropSequenceSQL(sequence.GetQuotedName(a)))
		}

		for _, sequence := range diff.GetCreatedSequences() {
			sql = append(sql, a.GetCreateSequenceSQL(sequence))
		}
	}

	sql = append(sql, a.GetCreateTablesSQL(diff.GetCreatedTables())...)
	sql = append(sql, a.GetDropTablesSQL(diff.GetDroppedTables())...)

	for _, tableDiff := range diff.GetAlteredTables() {
		sql = append(sql, a.GetAlterTableSQL(tableDiff)...)
	}

	return sql
}
func (parent *AbstractPlatform) GetCreateSequenceSQL(sequence *assets.Sequence) string {
	panic("Not supported")
}
func (parent *AbstractPlatform) GetAlterSequenceSQL(sequence *assets.Sequence) string {
	panic("Not supported")
}
func (parent *AbstractPlatform) GetDropSequenceSQL(name string) string {
	a := parent.child
	if !a.SupportsSequences() {
		panic("Not supported")
	}

	return `DROP SEQUENCE ` + name
}
func (parent *AbstractPlatform) GetCreateIndexSQL(
	index *assets.Index,
	table string,
) string {
	a := parent.child
	name := index.GetQuotedName(a)
	columns := index.GetColumns()

	if len(columns) == 0 {
		panic(
			fmt.Errorf(
				`incomplete or invalid index definition %s on table %s`,
				name,
				table,
			),
		)
	}

	if index.IsPrimary() {
		return a.GetCreatePrimaryKeySQL(index, table)
	}

	query := `CREATE ` + a.GetCreateIndexSQLFlags(index) + `INDEX ` + name + ` ON ` + table
	query += ` (` + strings.Join(
		index.GetQuotedColumns(a),
		`, `,
	) + `)` + a.GetPartialIndexSQL(index)

	return query
}
func (parent *AbstractPlatform) GetPartialIndexSQL(index *assets.Index) string {
	a := parent.child
	if a.SupportsPartialIndexes() && index.HasOption(`where`) {
		return ` WHERE ` + index.GetOption(`where`).(string)
	}

	return ``
}
func (parent *AbstractPlatform) GetCreateIndexSQLFlags(index *assets.Index) string {
	if index.IsUnique() {
		return `UNIQUE `
	} else {
		return ``
	}
}
func (parent *AbstractPlatform) GetCreatePrimaryKeySQL(
	index *assets.Index,
	table string,
) string {
	a := parent.child
	return `ALTER TABLE ` + table + ` ADD PRIMARY KEY (` + strings.Join(
		index.GetQuotedColumns(a),
		`, `,
	) + `)`
}
func (parent *AbstractPlatform) GetCreateSchemaSQL(schemaName string) string {
	a := parent.child
	if !a.SupportsSchemas() {
		panic("Not supported")
	}

	return `CREATE SCHEMA ` + schemaName
}
func (parent *AbstractPlatform) GetCreateUniqueConstraintSQL(
	constraint *assets.UniqueConstraint,
	tableName string,
) string {
	a := parent.child
	return `ALTER TABLE ` + tableName + ` ADD CONSTRAINT ` + constraint.GetQuotedName(a) + ` UNIQUE` + ` (` + strings.Join(
		constraint.GetQuotedColumns(a),
		`, `,
	) + `)`
}
func (parent *AbstractPlatform) GetDropSchemaSQL(schemaName string) string {
	a := parent.child
	if !a.SupportsSchemas() {
		panic("Not supported")
	}

	return `DROP SCHEMA ` + schemaName
}
func (parent *AbstractPlatform) QuoteIdentifier(identifier string) string {
	a := parent.child
	if strings.Contains(identifier, `.`) {
		parts := make([]string, 0)

		for _, s := range strings.Split(identifier, `.`) {
			parts = append(parts, a.QuoteSingleIdentifier(s))
		}

		return strings.Join(parts, `.`)
	}

	return a.QuoteSingleIdentifier(identifier)
}
func (parent *AbstractPlatform) QuoteSingleIdentifier(str string) string {
	return `"` + strings.ReplaceAll(str, `"`, `""`) + `"`
}
func (parent *AbstractPlatform) GetCreateForeignKeySQL(
	foreignKey *assets.ForeignKeyConstraint,
	table string,
) string {
	a := parent.child
	return `ALTER TABLE ` + table + ` ADD ` + a.GetForeignKeyDeclarationSQL(foreignKey)
}

func (parent *AbstractPlatform) GetRenameTableSQL(
	oldName string,
	newName string,
) string {
	return fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, oldName, newName)
}
func (parent *AbstractPlatform) GetPreAlterTableIndexForeignKeySQL(diff *diff_dtos.TableDiff) []string {
	a := parent.child
	tableNameSQL := diff.GetOldTable().GetQuotedName(a)

	sql := make([]string, 0)

	for _, foreignKey := range diff.GetDroppedForeignKeys() {
		sql = append(
			sql,
			a.GetDropForeignKeySQL(foreignKey.GetQuotedName(a), tableNameSQL),
		)
	}

	for _, foreignKey := range diff.GetModifiedForeignKeys() {
		sql = append(
			sql,
			a.GetDropForeignKeySQL(foreignKey.GetQuotedName(a), tableNameSQL),
		)
	}

	for _, index := range diff.GetDroppedIndexes() {
		sql = append(
			sql,
			a.GetDropIndexSQL(index.GetQuotedName(a), tableNameSQL),
		)
	}

	for _, index := range diff.GetModifiedIndexes() {
		sql = append(
			sql,
			a.GetDropIndexSQL(index.GetQuotedName(a), tableNameSQL),
		)
	}

	return sql
}
func (parent *AbstractPlatform) GetPostAlterTableIndexForeignKeySQL(diff *diff_dtos.TableDiff) []string {
	a := parent.child
	sql := make([]string, 0)

	tableNameSQL := diff.GetOldTable().GetQuotedName(a)

	for _, foreignKey := range diff.GetAddedForeignKeys() {
		sql = append(sql, a.GetCreateForeignKeySQL(foreignKey, tableNameSQL))
	}

	for _, foreignKey := range diff.GetModifiedForeignKeys() {
		sql = append(sql, a.GetCreateForeignKeySQL(foreignKey, tableNameSQL))
	}

	for _, index := range diff.GetAddedIndexes() {
		sql = append(sql, a.GetCreateIndexSQL(index, tableNameSQL))
	}

	for _, index := range diff.GetModifiedIndexes() {
		sql = append(sql, a.GetCreateIndexSQL(index, tableNameSQL))
	}

	for oldIndexName, index := range diff.GetRenamedIndexes() {
		oldIndexName := assets.NewIdentifier(oldIndexName)
		sql = append(
			sql,
			a.GetRenameIndexSQL(
				oldIndexName.GetQuotedName(a),
				index,
				tableNameSQL,
			)...,
		)
	}

	return sql
}
func (parent *AbstractPlatform) GetRenameIndexSQL(
	oldIndexName string,
	index *assets.Index,
	tableName string,
) []string {
	a := parent.child
	sql := make([]string, 0)

	sql = append(sql, a.GetDropIndexSQL(oldIndexName, tableName))
	sql = append(sql, a.GetCreateIndexSQL(index, tableName))

	return sql
}
func (parent *AbstractPlatform) GetRenameColumnSQL(
	tableName string,
	oldColumnName string,
	newColumnName string,
) []string {
	return []string{
		fmt.Sprintf(
			`ALTER TABLE %s RENAME COLUMN %s TO %s`,
			tableName,
			oldColumnName,
			newColumnName,
		),
	}
}
func (parent *AbstractPlatform) GetColumnDeclarationListSQL(columns []map[string]any) string {
	a := parent.child
	declarations := make([]string, 0)

	for _, column := range columns {
		declarations = append(
			declarations,
			a.GetColumnDeclarationSQL(column[`name`].(string), column),
		)
	}

	return strings.Join(declarations, `, `)
}

func (parent *AbstractPlatform) GetColumnDeclarationSQL(name string, column map[string]any) string {
	a := parent.child
	declaration := ""
	if v, ok := column[`columnDefinition`]; ok && v != nil && v.(*string) != nil && *(v.(*string)) != "" {
		declaration = v.(string)
	} else {
		defaultValue := a.GetDefaultValueDeclarationSQL(column)

		charset := ""
		if v, ok := column[`charset`]; ok {
			charset = ` ` + a.GetColumnCharsetDeclarationSQL(v.(string))
		}

		collation := ""
		if v, ok := column[`collation`]; ok {
			collation = ` ` + a.GetColumnCollationDeclarationSQL(v.(string))
		}

		notnull := ""
		if v, ok := column[`notnull`]; ok && v == true {
			notnull = ` NOT NULL`
		}

		typeVarDecl := column[`type`].(types.AbstractTypeInterface).GetSQLDeclaration(
			column,
			a,
		)
		declaration = typeVarDecl + charset + defaultValue + notnull + collation

		comment, hasComment := column[`comment`]
		if a.SupportsInlineColumnComments() && hasComment && comment != `` {
			declaration += ` ` + a.GetInlineColumnCommentSQL(comment.(string))
		}
	}

	return name + ` ` + declaration
}
func (parent *AbstractPlatform) GetDecimalTypeDeclarationSQL(column map[string]any) string {
	precision, hasPrecision := column[`precision`]
	scale, hasScale := column[`scale`]

	e := ""
	if !hasPrecision || precision.(*int) == nil {
		columnName := column[`name`].(string)
		e = fmt.Sprintf("column %s precision required", columnName)
	} else if !hasScale || scale.(*int) == nil {
		columnName := column[`name`].(string)
		e = fmt.Sprintf("column %s scale required", columnName)
	}
	if e != "" {
		panic("InvalidColumnDeclaration " + e)
	}

	precisionStr := strconv.Itoa(*(precision.(*int)))
	scaleStr := strconv.Itoa(*(scale.(*int)))

	return `NUMERIC(` + precisionStr + `, ` + scaleStr + `)`
}

func (parent *AbstractPlatform) GetDefaultValueDeclarationSQL(column map[string]any) string {
	a := parent.child
	defaultColumn, ok := column[`default`]
	if !ok || defaultColumn == "" {
		if _, ok2 := column[`notnull`]; !ok2 {
			return ` DEFAULT NULL`
		}

		return ``
	}

	defaultType := defaultColumn.(string)

	if _, ok := column[`type`]; !ok {
		return " DEFAULT `" + defaultType + "`"
	}

	typeVar := column[`type`].(types.AbstractTypeInterface)

	if _, ok := typeVar.(types.PhpIntegerMappingType); ok {
		return ` DEFAULT ` + defaultType
	}

	if _, ok := typeVar.(types.PhpDateTimeMappingType); ok && defaultType == a.GetCurrentTimestampSQL() {
		return ` DEFAULT ` + a.GetCurrentTimestampSQL()
	}

	if _, ok := typeVar.(types.PhpTimeMappingType); ok && defaultType == a.GetCurrentTimeSQL() {
		return ` DEFAULT ` + a.GetCurrentTimeSQL()
	}

	if _, ok := typeVar.(types.PhpDateMappingType); ok && defaultType == a.GetCurrentDateSQL() {
		return ` DEFAULT ` + a.GetCurrentDateSQL()
	}

	if _, ok := typeVar.(*types.BooleanType); ok {
		return ` DEFAULT ` + a.ConvertBooleans(defaultType).(string)
	}

	// Number
	_, err1 := strconv.ParseInt(defaultType, 10, 64)
	_, err2 := strconv.ParseFloat(defaultType, 64)
	if err1 == nil || err2 == nil {
		return ` DEFAULT ` + defaultType
	}

	return ` DEFAULT ` + a.QuoteStringLiteral(defaultType)
}
func (parent *AbstractPlatform) GetCheckDeclarationSQL(definition []map[string]any) string {
	constraints := make([]string, 0)
	for _, def := range definition {
		if v, ok := def[`min`]; ok {
			constraints = append(
				constraints,
				`CHECK (`+def[`name`].(string)+` >= `+v.(string)+`)`,
			)
		}

		if v, ok := def[`max`]; ok {
			constraints = append(
				constraints,
				`CHECK (`+def[`name`].(string)+` <= `+v.(string)+`)`,
			)
		}
	}

	return strings.Join(constraints, `, `)
}
func (parent *AbstractPlatform) GetUniqueConstraintDeclarationSQL(constraint *assets.UniqueConstraint) string {
	a := parent.child
	columns := constraint.GetColumns()

	if len(columns) == 0 {
		panic(`Incomplete definition. "columns" required.`)
	}

	chunks := []string{`CONSTRAINT`}

	if constraint.GetName() != `` {
		chunks = append(chunks, constraint.GetQuotedName(a))
	}

	chunks = append(chunks, `UNIQUE`)

	if constraint.HasFlag(`clustered`) {
		chunks = append(chunks, `CLUSTERED`)
	}

	chunks = append(chunks, fmt.Sprintf(`(%s)`, strings.Join(columns, `, `)))

	return strings.Join(chunks, ` `)
}

func (parent *AbstractPlatform) GetIndexDeclarationSQL(index *assets.Index) string {
	a := parent.child
	columns := index.GetColumns()

	if len(columns) == 0 {
		panic(`Incomplete definition. "columns" required.`)
	}

	return a.GetCreateIndexSQLFlags(index) + `INDEX ` + index.GetQuotedName(a) + ` (` + strings.Join(
		index.GetQuotedColumns(a),
		`, `,
	) + `)` + a.GetPartialIndexSQL(index)
}

func (parent *AbstractPlatform) GetTemporaryTableName(tableName string) string {
	return tableName
}

func (parent *AbstractPlatform) GetForeignKeyDeclarationSQL(foreignKey *assets.ForeignKeyConstraint) string {
	a := parent.child
	sql := a.GetForeignKeyBaseDeclarationSQL(foreignKey)
	sql += a.GetAdvancedForeignKeyOptionsSQL(foreignKey)

	return sql
}

func (parent *AbstractPlatform) GetAdvancedForeignKeyOptionsSQL(foreignKey *assets.ForeignKeyConstraint) string {
	a := parent.child
	query := ``
	if foreignKey.HasOption(`onUpdate`) && foreignKey.GetOption(`onUpdate`).(*string) != nil {
		query += ` ON UPDATE ` + a.GetForeignKeyReferentialActionSQL(*foreignKey.GetOption(`onUpdate`).(*string))
	}

	if foreignKey.HasOption(`onDelete`) && foreignKey.GetOption(`onDelete`).(*string) != nil {
		query += ` ON DELETE ` + a.GetForeignKeyReferentialActionSQL(*foreignKey.GetOption(`onDelete`).(*string))
	}

	return query
}

func (parent *AbstractPlatform) GetForeignKeyReferentialActionSQL(action string) string {
	upper := strings.ToUpper(action)

	switch upper {
	case `CASCADE`, `SET NULL`, `NO ACTION`, `RESTRICT`, `SET DEFAULT`:
		return upper
	default:
		panic(fmt.Errorf(`invalid foreign key action "%s"`, upper))
	}
}
func (parent *AbstractPlatform) GetForeignKeyBaseDeclarationSQL(foreignKey *assets.ForeignKeyConstraint) string {
	a := parent.child
	sql := ""
	if foreignKey.GetName() != `` {
		sql += `CONSTRAINT ` + foreignKey.GetQuotedName(a) + ` `
	}

	sql += `FOREIGN KEY (`

	if len(foreignKey.GetLocalColumns()) == 0 {
		panic(`Incomplete definition. "local" required.`)
	}

	if len(foreignKey.GetForeignColumns()) == 0 {
		panic(`Incomplete definition. "foreign" required.`)
	}

	if len(foreignKey.GetForeignTableName()) == 0 {
		panic(`Incomplete definition. "foreignTable" required.`)
	}

	return sql + strings.Join(
		foreignKey.GetQuotedLocalColumns(a),
		`, `,
	) + `) REFERENCES ` + foreignKey.GetQuotedForeignTableName(a) + ` (` + strings.Join(
		foreignKey.GetQuotedForeignColumns(a),
		`, `,
	) + `)`
}

func (parent *AbstractPlatform) GetColumnCharsetDeclarationSQL(charset string) string {
	return ``
}

func (parent *AbstractPlatform) GetColumnCollationDeclarationSQL(collation string) string {
	a := parent.child
	if a.SupportsColumnCollation() {
		return `COLLATE ` + a.QuoteSingleIdentifier(collation)
	}

	return ``
}
func (parent *AbstractPlatform) ConvertBooleans(item any) any {
	if itemMap, ok := item.(map[string]any); ok {
		for k, v := range itemMap {
			if _, ok := v.(bool); !ok {
				continue
			}

			itemMap[k] = v.(int)
		}
		return itemMap
	} else if itemSlice, ok := item.([]any); ok {
		for k, v := range itemSlice {
			if _, ok := v.(bool); !ok {
				continue
			}

			itemSlice[k] = v.(int)
		}

		return itemSlice
	} else if _, ok := item.(bool); ok {
		return item.(int)
	}

	return item
}
func (parent *AbstractPlatform) ConvertFromBoolean(item any) *bool {
	if item == nil {
		return nil
	}

	return item.(*bool)
}
func (parent *AbstractPlatform) ConvertBooleansToDatabaseValue(item any) any {
	a := parent.child
	return a.ConvertBooleans(item)
}
func (parent *AbstractPlatform) GetCurrentDateSQL() string {
	return `CURRENT_DATE`
}
func (parent *AbstractPlatform) GetCurrentTimeSQL() string {
	return `CURRENT_TIME`
}
func (parent *AbstractPlatform) GetCurrentTimestampSQL() string {
	return `CURRENT_TIMESTAMP`
}

func (parent *AbstractPlatform) GetListDatabasesSQL() string {
	panic("Not supported")
}

func (parent *AbstractPlatform) GetListSequencesSQL(database string) string {
	panic("Not supported")
}

func (parent *AbstractPlatform) GetCreateViewSQL(
	name string,
	sql string,
) string {
	return `CREATE VIEW ` + name + ` AS ` + sql
}
func (parent *AbstractPlatform) GetDropViewSQL(name string) string {
	return `DROP VIEW ` + name
}
func (parent *AbstractPlatform) GetSequenceNextValSQL(sequence string) string {
	panic("Not supported")
}
func (parent *AbstractPlatform) GetCreateDatabaseSQL(name string) string {
	return `CREATE ` + `DATABASE ` + name
}
func (parent *AbstractPlatform) GetDropDatabaseSQL(name string) string {
	return `DROP ` + `DATABASE ` + name
}

func (parent *AbstractPlatform) GetDateTimeTzTypeDeclarationSQL(column map[string]any) string {
	a := parent.child
	return a.GetDateTimeTypeDeclarationSQL(column)
}

func (parent *AbstractPlatform) GetFloatTypeDeclarationSQL(column map[string]any) string {
	return `DOUBLE PRECISION`
}
func (parent *AbstractPlatform) GetSmallFloatTypeDeclarationSQL(column map[string]any) string {
	return `REAL`
}
func (parent *AbstractPlatform) SupportsSequences() bool {
	return false
}
func (parent *AbstractPlatform) SupportsIdentityColumns() bool {
	return false
}

func (parent *AbstractPlatform) SupportsPartialIndexes() bool {
	return false
}
func (parent *AbstractPlatform) SupportsColumnLengthIndexes() bool {
	return false
}
func (parent *AbstractPlatform) SupportsSavepoints() bool {
	return true
}
func (parent *AbstractPlatform) SupportsReleaseSavepoints() bool {
	a := parent.child
	return a.SupportsSavepoints()
}
func (parent *AbstractPlatform) SupportsSchemas() bool {
	return false
}

func (parent *AbstractPlatform) SupportsInlineColumnComments() bool {
	return false
}

func (parent *AbstractPlatform) SupportsCommentOnStatement() bool {
	return false
}

func (parent *AbstractPlatform) SupportsColumnCollation() bool {
	return false
}
func (parent *AbstractPlatform) GetDateTimeFormatString() string {
	return `Y-m-d H:i:s`
}
func (parent *AbstractPlatform) GetDateTimeTzFormatString() string {
	return `Y-m-d H:i:s`
}
func (parent *AbstractPlatform) GetDateFormatString() string {
	return `Y-m-d`
}
func (parent *AbstractPlatform) GetTimeFormatString() string {
	return `H:i:s`
}
func (parent *AbstractPlatform) ModifyLimitQuery(
	query string,
	limit *int,
	offset int,
) (string, error) {
	a := parent.child
	if offset < 0 {
		panic(
			fmt.Sprintf(
				`Offset must be a positive integer or zero, %d given.`,
				offset,
			),
		)
	}

	return a.DoModifyLimitQuery(query, limit, offset), nil
}
func (parent *AbstractPlatform) DoModifyLimitQuery(
	query string,
	limit *int,
	offset int,
) string {
	if limit != nil {
		query += fmt.Sprintf(` LIMIT %d`, *limit)
	}

	if offset > 0 {
		query += fmt.Sprintf(` OFFSET %d`, offset)
	}

	return query
}
func (parent *AbstractPlatform) GetMaxIdentifierLength() int {
	return 63
}
func (parent *AbstractPlatform) GetEmptyIdentityInsertSQL(
	quotedTableName string,
	quotedIdentifierColumnName string,
) string {
	return `INSERT ` + `INTO ` + quotedTableName + ` (` + quotedIdentifierColumnName + `) VALUES (null)`
}
func (parent *AbstractPlatform) GetTruncateTableSQL(
	tableName string,
	cascade bool,
) string {
	a := parent.child
	tableIdentifier := assets.NewIdentifier(tableName)

	return `TRUNCATE ` + tableIdentifier.GetQuotedName(a)
}
func (parent *AbstractPlatform) GetDummySelectSQL(expression string) string {
	return fmt.Sprintf(`SELECT %s`, expression)
}
func (parent *AbstractPlatform) CreateSavePoint(savepoint string) string {
	return `SAVEPOINT ` + savepoint
}
func (parent *AbstractPlatform) ReleaseSavePoint(savepoint string) string {
	return `RELEASE SAVEPOINT ` + savepoint
}
func (parent *AbstractPlatform) RollbackSavePoint(savepoint string) string {
	return `ROLLBACK TO SAVEPOINT ` + savepoint
}
func (parent *AbstractPlatform) GetReservedKeywordsList() keywords.KeywordListInterface {
	a := parent.child
	// Store the instance so it doesn`t need to be generated on every request.
	if parent.keywords == nil {
		parent.keywords = a.CreateReservedKeywordsList()
	}
	return parent.keywords
}

func (parent *AbstractPlatform) QuoteStringLiteral(str string) string {
	return "'" + strings.ReplaceAll(str, "'", "''") + "'"
}

func (parent *AbstractPlatform) EscapeStringForLike(inputString string, escapeChar string) string {
	a := parent.child
	quoted := utils.Quote(a.GetLikeWildcardCharacters()+escapeChar, `~`)

	r := regexp.MustCompile(`~([` + quoted + `])~u`)
	sql := r.ReplaceAllString(utils.AddSlashes(escapeChar)+`1`, inputString)

	return sql
}

func (parent *AbstractPlatform) ColumnToArray(column *assets.Column) map[string]any {
	a := parent.child
	arr := column.ToArray()
	arr[`name`] = column.GetQuotedName(a)

	var version any
	if column.HasPlatformOption(`version`) {
		version = column.GetPlatformOption(`version`)
	} else {
		version = false
	}
	arr["version"] = version

	arr[`comment`] = column.GetComment()

	return arr
}
func (parent *AbstractPlatform) GetLikeWildcardCharacters() string {
	return `%_`
}
func (parent *AbstractPlatform) ColumnsEqual(
	column1 *assets.Column,
	column2 *assets.Column,
) bool {
	a := parent.child
	column1Array := a.ColumnToArray(column1)
	column2Array := a.ColumnToArray(column2)

	// ignore explicit columnDefinition since it`s not set on the Column generated by the SchemaManager
	delete(column1Array, "columnDefinition")
	delete(column2Array, "columnDefinition")

	if a.GetColumnDeclarationSQL(
		``,
		column1Array,
	) != a.GetColumnDeclarationSQL(``, column2Array) {
		return false
	}

	// If the platform supports inline comments, all comparison is already done above
	if a.SupportsInlineColumnComments() {
		return true
	}

	return column1.GetComment() == column2.GetComment()
}
func (parent *AbstractPlatform) GetUnionSelectPartSQL(subQuery string) string {
	return fmt.Sprintf(`(%s)`, subQuery)
}
func (parent *AbstractPlatform) GetUnionAllSQL() string {
	return `UNION ALL`
}
func (parent *AbstractPlatform) GetUnionDistinctSQL() string {
	return `UNION`
}

func (parent *AbstractPlatform) GetAsciiStringTypeDeclarationSQL(column map[string]any) string {
	a := parent.child
	return a.GetStringTypeDeclarationSQL(column)
}
