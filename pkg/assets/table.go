package assets

import (
	"github.com/KoNekoD/gormite/pkg/dtos"
	"github.com/KoNekoD/gormite/pkg/types"
	"github.com/KoNekoD/smt/pkg/smt"
	"github.com/elliotchance/orderedmap/v3"
	"regexp"
	"slices"
	"strings"
)

// Table - Object Representation of a table.
type Table struct {
	*AbstractAsset

	columns *orderedmap.OrderedMap[string, *Column]

	implicitIndexes map[string]*Index

	// renamedColumns - keys are new names, values are old names
	renamedColumns map[string]string

	indexes map[string]*Index

	primaryKeyName *string

	uniqueConstraints map[string]*UniqueConstraint

	fkConstraints map[string]*ForeignKeyConstraint

	options map[string]any

	schemaConfig *dtos.SchemaConfig
}

func NewTable(
	name string,
	columns []*Column,
	indexes []*Index,
	uniqueConstraints []*UniqueConstraint,
	fkConstraints []*ForeignKeyConstraint,
	options map[string]any,
) *Table {
	if name == "" {
		panic("name must not be empty")
	}

	v := &Table{
		AbstractAsset:     NewAbstractAsset(),
		columns:           orderedmap.NewOrderedMap[string, *Column](),
		implicitIndexes:   make(map[string]*Index),
		renamedColumns:    make(map[string]string),
		indexes:           make(map[string]*Index),
		primaryKeyName:    nil,
		uniqueConstraints: make(map[string]*UniqueConstraint),
		fkConstraints:     make(map[string]*ForeignKeyConstraint),
		options: map[string]any{
			"createoptions": make(map[string]string),
		},
		schemaConfig: nil,
	}

	v.SetName(name)

	for _, column := range columns {
		v.addColumn(column)
	}

	for _, idx := range indexes {
		v.addIndex(idx)
	}

	for _, uniqueConstraint := range uniqueConstraints {
		v.addUniqueConstraint(uniqueConstraint)
	}

	for _, fkConstraint := range fkConstraints {
		v.addForeignKeyConstraint(fkConstraint)
	}

	v.options = options

	return v
}

func (t *Table) SetSchemaConfig(schemaConfig *dtos.SchemaConfig) {
	t.schemaConfig = schemaConfig
}

// SetPrimaryKey - Sets the Primary Key.
// indexName by default nil
func (t *Table) SetPrimaryKey(columnNames []string, indexName *string) *Table {
	if indexName == nil {
		indexName = new(string)
		*indexName = "primary"
	}

	t.addIndex(
		t.createIndex(
			columnNames,
			*indexName,
			true,
			true,
			make([]string, 0),
			make(map[string]any),
		),
	)

	for _, columnName := range columnNames {
		column := t.GetColumn(columnName)
		column.SetNotNull()
	}

	return t
}

func (t *Table) AddUniqueConstraint(
	columnNames []string,
	indexName *string,
	flags []string,
	options map[string]string,
) *Table {
	if indexName == nil {
		columns := make([]string, 0)
		columns = append(columns, t.GetName())
		columns = append(columns, columnNames...)
		indexName = new(string)
		*indexName = t.generateIdentifierName(
			columns,
			"uniq",
			t.getMaxIdentifierLength(),
		)
	}

	return t.addUniqueConstraint(
		t.createUniqueConstraint(
			columnNames,
			*indexName,
			flags,
			options,
		),
	)
}

func (t *Table) AddIndex(columnNames []string, indexName *string, flags []string, options map[string]any) *Table {
	if indexName == nil || len(*indexName) > t.getMaxIdentifierLength() {
		columns := make([]string, 0)
		columns = append(columns, t.GetName())
		columns = append(columns, columnNames...)
		indexName = new(string)
		*indexName = t.generateIdentifierName(
			columns,
			"idx",
			t.getMaxIdentifierLength(),
		)
	}

	return t.addIndex(
		t.createIndex(
			columnNames,
			*indexName,
			false,
			false,
			flags,
			options,
		),
	)
}

// DropPrimaryKey - Drops the primary key from this table.
func (t *Table) DropPrimaryKey() {
	if t.primaryKeyName == nil {
		return
	}

	t.DropIndex(*t.primaryKeyName)

	t.primaryKeyName = nil
}

// DropIndex - Drops an index from this table.
func (t *Table) DropIndex(name string) {
	name = t.normalizeIdentifier(&name)

	if !t.HasIndex(name) {
		panic("index " + name + " not found")
	}

	delete(t.indexes, name)
}

func (t *Table) AddUniqueIndex(columnNames []string, indexName *string, options map[string]any) *Table {
	if indexName == nil || len(*indexName) > t.getMaxIdentifierLength() {
		columns := make([]string, 0)
		columns = append(columns, t.GetName())
		columns = append(columns, columnNames...)
		indexName = new(string)
		*indexName = t.generateIdentifierName(
			columns,
			"uniq",
			t.getMaxIdentifierLength(),
		)
	}

	return t.addIndex(
		t.createIndex(
			columnNames,
			*indexName,
			true,
			false,
			make([]string, 0),
			options,
		),
	)
}

// RenameIndex - Renames an index.
// @param string      oldName The name of the index to rename from.
// @param string|nil newName The name of the index to rename to. If nil is given, the index name will be auto-generated.
func (t *Table) RenameIndex(oldName string, newName *string) *Table {
	oldName = t.normalizeIdentifier(&oldName)
	normalizedNewName := t.normalizeIdentifier(newName)

	if oldName == normalizedNewName {
		return t
	}

	if !t.HasIndex(oldName) {
		panic("index " + oldName + " not found")
	}

	if t.HasIndex(normalizedNewName) {
		panic("index " + normalizedNewName + " already exists")
	}

	oldIndex, ok := t.indexes[oldName]

	if !ok {
		panic("index " + oldName + " not found")
	}

	if oldIndex.IsPrimary() {
		t.DropPrimaryKey()

		return t.SetPrimaryKey(oldIndex.GetColumns(), newName)
	}

	delete(t.indexes, oldName)

	if oldIndex.IsUnique() {
		return t.AddUniqueIndex(
			oldIndex.GetColumns(),
			newName,
			oldIndex.GetOptions(),
		)
	}

	return t.AddIndex(
		oldIndex.GetColumns(),
		newName,
		oldIndex.GetFlags(),
		oldIndex.GetOptions(),
	)
}

// ColumnsAreIndexed - Checks if an index begins in the order of the given columns.
func (t *Table) ColumnsAreIndexed(columnNames []string) bool {
	for _, index := range t.GetIndexes() {
		if index.SpansColumns(columnNames) {
			return true
		}
	}

	return false
}

func (t *Table) AddColumn(
	name string,
	typeName types.AbstractTypeInterface,
	options ...ColumnOption,
) *Column {
	column := NewColumn(name, typeName, options...)

	t.addColumn(column)

	return column
}

func (t *Table) GetRenamedColumns() map[string]string {
	return t.renamedColumns
}

func (t *Table) RenameColumn(oldName string, newName string) *Column {
	oldName = t.normalizeIdentifier(&oldName)
	newName = t.normalizeIdentifier(&newName)

	if oldName == newName {
		panic("column " + oldName + " already exists")
	}

	column := t.GetColumn(oldName)
	column.SetName(newName)
	t.columns.Delete(oldName)
	t.addColumn(column)

	// If a column is renamed multiple times, we only want to know the original and last new name
	if _, ok := t.renamedColumns[oldName]; ok {
		toRemove := oldName
		oldName = t.renamedColumns[oldName]
		delete(t.renamedColumns, toRemove)
	}

	if newName != oldName {
		t.renamedColumns[newName] = oldName
	}

	return column
}

func (t *Table) ModifyColumn(name string, options map[string]any) *Table {
	column := t.GetColumn(name)

	if len(options) > 0 {
		panic("Not implemented")
	}

	column.SetOptions(nil)

	return t
}

// DropColumn - Drops a Column from the Table.
func (t *Table) DropColumn(name string) *Table {
	name = t.normalizeIdentifier(&name)

	t.columns.Delete(name)

	return t
}

// AddForeignKeyConstraint - Adds a foreign key constraint.
// Name is inferred from the local columns.
func (t *Table) AddForeignKeyConstraint(
	foreignTableName string,
	localColumnNames []string,
	foreignColumnNames []string,
	options map[string]any,
	name *string,
) *Table {
	if name == nil {
		columns := make([]string, 0)
		columns = append(columns, t.GetName())
		columns = append(columns, localColumnNames...)
		name = new(string)
		*name = t.generateIdentifierName(
			columns,
			"fk",
			t.getMaxIdentifierLength(),
		)
	}

	for _, columnName := range localColumnNames {
		if !t.HasColumn(columnName) {
			panic("column does not exist: " + columnName)
		}
	}

	constraint := NewForeignKeyConstraint(
		*name,
		localColumnNames,
		foreignTableName,
		foreignColumnNames,
		options,
	)

	return t.addForeignKeyConstraint(constraint)
}

func (t *Table) AddOption(name string, value any) *Table {
	t.options[name] = value

	return t
}

// HasForeignKey - Returns whether this table has a foreign key constraint with the given name.
func (t *Table) HasForeignKey(name string) bool {
	name = t.normalizeIdentifier(&name)

	_, ok := t.fkConstraints[name]

	return ok
}

// GetForeignKey - Returns the foreign key constraint with the given name.
func (t *Table) GetForeignKey(name string) *ForeignKeyConstraint {
	name = t.normalizeIdentifier(&name)

	if !t.HasForeignKey(name) {
		panic("foreign key " + name + " not found")
	}

	v, ok := t.fkConstraints[name]
	if !ok {
		panic("not found")
	}

	return v
}

// RemoveForeignKey - Removes the foreign key constraint with the given name.
func (t *Table) RemoveForeignKey(name string) {
	name = t.normalizeIdentifier(&name)

	if !t.HasForeignKey(name) {
		panic("foreign key " + name + " not found")
	}

	delete(t.fkConstraints, name)
}

// HasUniqueConstraint - Returns whether this table has a unique constraint with the given name.
func (t *Table) HasUniqueConstraint(name string) bool {
	name = t.normalizeIdentifier(&name)

	_, ok := t.uniqueConstraints[name]

	return ok
}

// GetUniqueConstraint - Returns the unique constraint with the given name.
func (t *Table) GetUniqueConstraint(name string) *UniqueConstraint {
	name = t.normalizeIdentifier(&name)

	if !t.HasUniqueConstraint(name) {
		panic("unique constraint " + name + " not found")
	}

	v, ok := t.uniqueConstraints[name]
	if !ok {
		panic("not found")
	}

	return v
}

// RemoveUniqueConstraint - Removes the unique constraint with the given name.
func (t *Table) RemoveUniqueConstraint(name string) {
	name = t.normalizeIdentifier(&name)

	if !t.HasUniqueConstraint(name) {
		panic("unique constraint " + name + " not found")
	}

	delete(t.uniqueConstraints, name)
}

func (t *Table) GetColumns() []*Column {
	return smt.IterToSlice(t.columns.Values())
}

// HasColumn - Returns whether this table has a Column with the given name.
func (t *Table) HasColumn(name string) bool {
	name = t.normalizeIdentifier(&name)

	_, ok := t.columns.Get(name)

	return ok
}

// GetColumn - Returns the Column with the given name.
func (t *Table) GetColumn(name string) *Column {
	name = t.normalizeIdentifier(&name)

	if !t.HasColumn(name) {
		panic("column " + name + " not found")
	}

	v, ok := t.columns.Get(name)
	if !ok {
		panic("not found")
	}

	return v
}

// GetPrimaryKey - Returns the primary key.
func (t *Table) GetPrimaryKey() *Index {
	if t.primaryKeyName != nil {
		return t.GetIndex(*t.primaryKeyName)
	}

	return nil
}

// HasIndex - Returns whether this table has an Index with the given name.
func (t *Table) HasIndex(name string) bool {
	name = t.normalizeIdentifier(&name)

	_, ok := t.indexes[name]

	return ok
}

// GetIndex - Returns the Index with the given name.
func (t *Table) GetIndex(name string) *Index {
	name = t.normalizeIdentifier(&name)

	if !t.HasIndex(name) {
		panic("index " + name + " not found")
	}

	v, ok := t.indexes[name]
	if !ok {
		panic("not found")
	}

	return v
}

func (t *Table) GetIndexes() map[string]*Index {
	return t.indexes
}

// GetUniqueConstraints - Returns the unique constraints.
func (t *Table) GetUniqueConstraints() map[string]*UniqueConstraint {
	return t.uniqueConstraints
}

// GetForeignKeys - Returns the foreign key constraints.
func (t *Table) GetForeignKeys() map[string]*ForeignKeyConstraint {
	return t.fkConstraints
}

func (t *Table) HasOption(name string) bool {
	_, ok := t.options[name]
	return ok
}

func (t *Table) GetOption(name string) any {
	v, ok := t.options[name]
	if !ok {
		panic("not found")
	}

	return v
}

func (t *Table) GetOptions() map[string]any {
	if t.options == nil { // TODO: ПОПРАВИТЬ
		return make(map[string]any)
	}

	return t.options
}

func (t *Table) getMaxIdentifierLength() int {
	if t.schemaConfig != nil {
		return t.schemaConfig.GetMaxIdentifierLength()
	}

	return 63
}

func (t *Table) addColumn(column *Column) {
	columnName := column.GetName()
	columnName = t.normalizeIdentifier(&columnName)

	if _, ok := t.columns.Get(columnName); ok {
		panic("column " + columnName + " already exists")
	}

	t.columns.Set(columnName, column)
}

// addIndex - Adds an index to the table.
func (t *Table) addIndex(indexCandidate *Index) *Table {
	indexName := indexCandidate.GetName()
	indexName = t.normalizeIdentifier(&indexName)
	replacedImplicitIndexes := make([]string, 0)

	for name, implicitIndex := range t.implicitIndexes {
		if !implicitIndex.IsFulfilledBy(indexCandidate) {
			continue
		}
		if _, ok := t.indexes[name]; !ok {
			continue
		}

		replacedImplicitIndexes = append(replacedImplicitIndexes, name)
	}

	_, hasIndex := t.indexes[indexName]
	if hasIndex && !slices.Contains(replacedImplicitIndexes, indexName) ||
		t.primaryKeyName != nil && indexCandidate.IsPrimary() {
		panic("index " + indexName + " already exists")
	}

	for _, name := range replacedImplicitIndexes {
		delete(t.indexes, name)
		delete(t.implicitIndexes, name)
	}

	if indexCandidate.IsPrimary() {
		t.primaryKeyName = &indexName
	}

	t.indexes[indexName] = indexCandidate

	return t
}

func (t *Table) addUniqueConstraint(constraint *UniqueConstraint) *Table {
	columns := make([]string, 0)
	columns = append(columns, t.GetName())
	columns = append(columns, constraint.GetColumns()...)

	name := constraint.GetName()
	if name == "" {
		name = t.generateIdentifierName(
			columns,
			"fk",
			t.getMaxIdentifierLength(),
		)
	}

	name = t.normalizeIdentifier(&name)

	t.uniqueConstraints[name] = constraint

	// If there is already an index that fulfills this requirements drop the request. In the case of __construct
	// calling this method during hydration from schema-details all the explicitly added indexes lead to duplicates.
	// This creates computation overhead in this case, however no duplicate indexes are ever added (column based).
	indexName := t.generateIdentifierName(
		columns,
		"idx",
		t.getMaxIdentifierLength(),
	)

	indexCandidate := t.createIndex(
		constraint.GetColumns(),
		indexName,
		true,
		false,
		make([]string, 0),
		make(map[string]any),
	)

	for _, existingIndex := range t.indexes {
		if indexCandidate.IsFulfilledBy(existingIndex) {
			return t
		}
	}

	t.implicitIndexes[t.normalizeIdentifier(&indexName)] = indexCandidate

	return t
}

func (t *Table) addForeignKeyConstraint(constraint *ForeignKeyConstraint) *Table {
	columns := make([]string, 0)
	columns = append(columns, t.GetName())
	columns = append(columns, constraint.GetLocalColumns()...)

	name := constraint.GetName()
	if name == "" {
		name = t.generateIdentifierName(
			columns,
			"fk",
			t.getMaxIdentifierLength(),
		)
	}

	name = t.normalizeIdentifier(&name)

	t.fkConstraints[name] = constraint

	// add an explicit index on the foreign key columns.
	// If there is already an index that fulfills this requirements drop the request. In the case of __construct
	// calling this method during hydration from schema-details all the explicitly added indexes lead to duplicates.
	// This creates computation overhead in this case, however no duplicate indexes are ever added (column based).
	indexName := t.generateIdentifierName(
		columns,
		"idx",
		t.getMaxIdentifierLength(),
	)

	indexCandidate := t.createIndex(
		constraint.GetLocalColumns(),
		indexName,
		false,
		false,
		make([]string, 0),
		make(map[string]any),
	)

	for _, existingIndex := range t.indexes {
		if indexCandidate.IsFulfilledBy(existingIndex) {
			return t
		}
	}

	t.addIndex(indexCandidate)
	t.implicitIndexes[t.normalizeIdentifier(&indexName)] = indexCandidate

	return t
}

/**
* Normalizes a given identifier.
*
* Trims quotes and lowercases the given identifier.
 */
func (t *Table) normalizeIdentifier(identifier *string) string {
	if identifier == nil {
		return ""
	}

	return t.trimQuotes(strings.ToLower(*identifier))
}

func (t *Table) SetComment(comment *string) *Table {
	// For keeping backward compatibility with MySQL in previous releases, table comments are stored as options.
	t.AddOption("comment", comment)

	return t
}

func (t *Table) GetComment() *string {
	v, ok := t.options["comment"]
	if !ok {
		panic("not found")
	}

	return v.(*string)
}

/**
* @param array<string|int, string> columns
* @param array<int, string>        flags
* @param array<string, mixed>      options
 */
func (t *Table) createUniqueConstraint(
	columns []string,
	indexName string,
	flags []string,
	options map[string]string,
) *UniqueConstraint {
	if matched, err := regexp.Match(
		"([^a-zA-Z0-9_]+)",
		[]byte(t.normalizeIdentifier(&indexName)),
	); matched || err != nil {
		panic("index name invalid: " + indexName)
	}

	columnName := ""

	for _, value := range columns {
		columnName = value

		if !t.HasColumn(columnName) {
			panic("column does not exist: " + columnName)
		}
	}

	return NewUniqueConstraint(
		indexName,
		columns,
		WithFlags(flags),
		WithOptions(options),
	)
}

func (t *Table) createIndex(
	columns []string,
	indexName string,
	isUnique bool,
	isPrimary bool,
	flags []string,
	options map[string]any,
) *Index {
	if matched, err := regexp.Match(
		"([^a-zA-Z0-9_]+)",
		[]byte(t.normalizeIdentifier(&indexName)),
	); matched || err != nil {
		panic("index name invalid: " + indexName)
	}

	for _, columnName := range columns {
		if !t.HasColumn(columnName) {
			panic("column does not exist: " + columnName)
		}
	}

	return NewIndex(indexName, columns, isUnique, isPrimary, flags, options)
}
