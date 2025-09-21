package diff_calc

import (
	"github.com/KoNekoD/gormite/pkg/utils"
	"maps"
	"slices"
	"strings"

	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/diff_dtos"
)

// Comparator - Compares two Schemas and return an instance of SchemaDiff.
type Comparator struct {
	platform DiffCalcPlatform
}

// NewComparator - The comparator can be only instantiated by a schema manager.
func NewComparator(platform DiffCalcPlatform) *Comparator {
	return &Comparator{platform: platform}
}

// CompareSchemas - Returns the differences between the schemas.
func (c *Comparator) CompareSchemas(oldSchema, newSchema *assets.Schema) *diff_dtos.SchemaDiff {
	createdSchemas := make([]string, 0)
	droppedSchemas := make([]string, 0)
	createdTables := make([]*assets.Table, 0)
	alteredTables := make([]*diff_dtos.TableDiff, 0)
	droppedTables := make([]*assets.Table, 0)
	createdSequences := make([]*assets.Sequence, 0)
	alteredSequences := make([]*assets.Sequence, 0)
	droppedSequences := make([]*assets.Sequence, 0)

	for _, newNamespace := range newSchema.GetNamespaces() {
		if oldSchema.HasNamespace(newNamespace) {
			continue
		}

		createdSchemas = append(createdSchemas, newNamespace)
	}

	for _, oldNamespace := range oldSchema.GetNamespaces() {
		if newSchema.HasNamespace(oldNamespace) {
			continue
		}

		droppedSchemas = append(droppedSchemas, oldNamespace)
	}

	for _, newTable := range newSchema.GetTables() {
		newTableName := newTable.GetShortestName(newSchema.GetName())
		if !oldSchema.HasTable(newTableName) {
			createdTables = append(createdTables, newSchema.GetTable(newTableName))
		} else {
			tableDiff := c.CompareTables(oldSchema.GetTable(newTableName), newSchema.GetTable(newTableName))

			if !tableDiff.IsEmpty() {
				alteredTables = append(alteredTables, tableDiff)
			}
		}
	}

	// Check if there are tables removed
	for _, oldTable := range oldSchema.GetTables() {
		oldTableName := oldTable.GetShortestName(oldSchema.GetName())

		oldTable = oldSchema.GetTable(oldTableName)
		if newSchema.HasTable(oldTableName) {
			continue
		}

		droppedTables = append(droppedTables, oldTable)
	}

	for _, newSequence := range newSchema.GetSequences() {
		newSequenceName := newSequence.GetShortestName(newSchema.GetName())

		if !oldSchema.HasSequence(newSequenceName) {
			if !c.isAutoIncrementSequenceInSchema(oldSchema, newSequence) {
				createdSequences = append(createdSequences, newSequence)
			}
		} else {
			if c.diffSequence(newSequence, oldSchema.GetSequence(newSequenceName)) {
				alteredSequences = append(alteredSequences, newSchema.GetSequence(newSequenceName))
			}
		}
	}

	for _, oldSequence := range oldSchema.GetSequences() {
		if c.isAutoIncrementSequenceInSchema(newSchema, oldSequence) {
			continue
		}

		oldSequenceName := oldSequence.GetShortestName(oldSchema.GetName())

		if newSchema.HasSequence(oldSequenceName) {
			continue
		}

		droppedSequences = append(droppedSequences, oldSequence)
	}

	return diff_dtos.NewSchemaDiff(
		createdSchemas,
		droppedSchemas,
		createdTables,
		alteredTables,
		droppedTables,
		createdSequences,
		alteredSequences,
		droppedSequences,
	)
}

func (c *Comparator) isAutoIncrementSequenceInSchema(schema *assets.Schema, sequence *assets.Sequence) bool {
	for _, table := range schema.GetTables() {
		if sequence.IsAutoIncrementsFor(table) {
			return true
		}
	}

	return false
}

func (c *Comparator) diffSequence(sequence1 *assets.Sequence, sequence2 *assets.Sequence) bool {
	if sequence1.GetAllocationSize() != sequence2.GetAllocationSize() {
		return true
	}

	return sequence1.GetInitialValue() != sequence2.GetInitialValue()
}

// CompareTables - Compares the tables and returns the difference between them.
func (c *Comparator) CompareTables(oldTable, newTable *assets.Table) *diff_dtos.TableDiff {
	addedColumns := make(map[string]*assets.Column)
	modifiedColumns := make(map[string]*diff_dtos.ColumnDiff)
	droppedColumns := make(map[string]*assets.Column)
	addedIndexes := make(map[string]*assets.Index)
	modifiedIndexes := make([]*assets.Index, 0)
	droppedIndexes := make(map[string]*assets.Index)
	addedForeignKeys := make([]*assets.ForeignKeyConstraint, 0)
	modifiedForeignKeys := make([]*assets.ForeignKeyConstraint, 0)
	droppedForeignKeys := make([]*assets.ForeignKeyConstraint, 0)

	oldColumns := oldTable.GetColumns()
	newColumns := newTable.GetColumns()

	// See if all the columns in the old table exist in the new table
	for newColumn := range newColumns {
		newColumnName := strings.ToLower(newColumn.GetName())

		if oldTable.HasColumn(newColumnName) {
			continue
		}

		addedColumns[newColumnName] = newColumn
	}

	// See if there are any removed columns in the new table
	for oldColumn := range oldColumns {
		oldColumnName := strings.ToLower(oldColumn.GetName())
		if !newTable.HasColumn(oldColumnName) {
			droppedColumns[oldColumnName] = oldColumn
			continue
		}

		newColumn := newTable.GetColumn(oldColumnName)

		if c.columnsEqual(oldColumn, newColumn) {
			continue
		}

		modifiedColumns[oldColumnName] = diff_dtos.NewColumnDiff(oldColumn, newColumn)
	}

	renamedColumnNames := newTable.GetRenamedColumns()

	for addedColumnName, addedColumn := range addedColumns {
		if _, ok := renamedColumnNames[addedColumn.GetName()]; !ok {
			continue
		}

		removedColumnName := strings.ToLower(renamedColumnNames[addedColumn.GetName()])
		// Explicitly renamed columns need to be diffed, because their types can also have changed
		modifiedColumns[removedColumnName] = diff_dtos.NewColumnDiff(
			droppedColumns[removedColumnName],
			addedColumn,
		)

		delete(addedColumns, addedColumnName)
		delete(droppedColumns, removedColumnName)
	}

	c.detectRenamedColumns(modifiedColumns, addedColumns, droppedColumns)

	oldIndexes := oldTable.GetIndexes()
	newIndexes := newTable.GetIndexes()

	// See if all the indexes from the old table exist in the new one
	for newIndexName, newIndex := range newIndexes {
		if newIndex.IsPrimary() && oldTable.GetPrimaryKey() != nil || oldTable.HasIndex(newIndexName) {
			continue
		}

		addedIndexes[newIndexName] = newIndex
	}

	// See if there are any removed indexes in the new table
	for oldIndexName, oldIndex := range oldIndexes {
		// See if the index is removed in the new table.
		if oldIndex.IsPrimary() && newTable.GetPrimaryKey() == nil || !oldIndex.IsPrimary() && !newTable.HasIndex(oldIndexName) {
			droppedIndexes[oldIndexName] = oldIndex
			continue
		}
		// See if index has changed in the new table.
		var newIndex *assets.Index
		if oldIndex.IsPrimary() {
			newIndex = newTable.GetPrimaryKey()
		} else {
			newIndex = newTable.GetIndex(oldIndexName)
		}
		if newIndex == nil {
			panic("assertion failed: new index is nil")
		}

		if !c.diffIndex(oldIndex, newIndex) {
			continue
		}

		modifiedIndexes = append(modifiedIndexes, newIndex)
	}

	renamedIndexes := c.detectRenamedIndexes(addedIndexes, droppedIndexes)

	oldForeignKeys := maps.Clone(oldTable.GetForeignKeys())
	newForeignKeys := maps.Clone(newTable.GetForeignKeys())

	for oldKey, oldForeignKey := range oldForeignKeys {
		for newKey, newForeignKey := range newForeignKeys {
			if c.diffForeignKey(oldForeignKey, newForeignKey) == false {
				delete(oldForeignKeys, oldKey)
				delete(newForeignKeys, newKey)
			} else {
				if strings.ToLower(oldForeignKey.GetName()) == strings.ToLower(newForeignKey.GetName()) {
					modifiedForeignKeys = append(
						modifiedForeignKeys,
						newForeignKey,
					)

					delete(oldForeignKeys, oldKey)
					delete(newForeignKeys, newKey)
				}
			}
		}
	}

	for _, oldForeignKey := range oldForeignKeys {
		droppedForeignKeys = append(droppedForeignKeys, oldForeignKey)
	}

	for _, newForeignKey := range newForeignKeys {
		addedForeignKeys = append(addedForeignKeys, newForeignKey)
	}

	return diff_dtos.NewTableDiff(
		oldTable,
		droppedForeignKeys,
		addedColumns,
		modifiedColumns,
		droppedColumns,
		addedIndexes,
		modifiedIndexes,
		droppedIndexes,
		renamedIndexes,
		addedForeignKeys,
		modifiedForeignKeys,
	)
}

// detectRenamedColumns - Try to find columns that only changed their name, rename operations maybe cheaper than add/drop
// however ambiguities between different possibilities should not lead to renaming at all.
func (c *Comparator) detectRenamedColumns(
	modifiedColumns map[string]*diff_dtos.ColumnDiff,
	addedColumns map[string]*assets.Column,
	removedColumns map[string]*assets.Column,
) {
	candidatesByName := make(map[string][][]*assets.Column)

	for addedColumnName, addedColumn := range addedColumns {
		for _, removedColumn := range removedColumns {
			if !c.columnsEqual(addedColumn, removedColumn) {
				continue
			}

			candidatesByName[addedColumnName] = append(
				candidatesByName[addedColumnName],
				[]*assets.Column{removedColumn, addedColumn},
			)
		}
	}

	for addedColumnName, candidates := range candidatesByName {
		if len(candidates) != 1 {
			continue
		}

		oldColumn, newColumn := candidates[0][0], candidates[0][1]
		oldColumnName := strings.ToLower(oldColumn.GetName())

		if _, ok := modifiedColumns[oldColumnName]; ok {
			continue
		}

		modifiedColumns[oldColumnName] = diff_dtos.NewColumnDiff(oldColumn, newColumn)

		delete(addedColumns, addedColumnName)
		delete(removedColumns, oldColumnName)
	}
}

// detectRenamedIndexes - Try to find indexes that only changed their name, rename operations maybe cheaper than add/drop
// however ambiguities between different possibilities should not lead to renaming at all.
func (c *Comparator) detectRenamedIndexes(
	addedIndexes map[string]*assets.Index,
	removedIndexes map[string]*assets.Index,
) map[string]*assets.Index {
	candidatesByName := make(map[string][][]*assets.Index)

	// Gather possible rename candidates by comparing each added and removed index based on semantics.
	for addedIndexName, addedIndex := range addedIndexes {
		for _, removedIndex := range removedIndexes {
			if c.diffIndex(addedIndex, removedIndex) {
				continue
			}

			candidatesByName[addedIndexName] = append(
				candidatesByName[addedIndexName],
				[]*assets.Index{removedIndex, addedIndex},
			)
		}
	}

	renamedIndexes := make(map[string]*assets.Index)

	for _, candidates := range candidatesByName {
		if len(candidates) != 1 {
			continue
		}

		removedIndex, addedIndex := candidates[0][0], candidates[0][1]

		removedIndexName := strings.ToLower(removedIndex.GetName())
		addedIndexName := strings.ToLower(addedIndex.GetName())

		if _, ok := renamedIndexes[removedIndexName]; ok {
			continue
		}

		renamedIndexes[removedIndexName] = addedIndex

		delete(addedIndexes, addedIndexName)
		delete(removedIndexes, removedIndexName)
	}

	return renamedIndexes
}

func (c *Comparator) diffForeignKey(key1 *assets.ForeignKeyConstraint, key2 *assets.ForeignKeyConstraint) bool {
	same := slices.Equal(
		utils.ToLowerList(key1.GetUnquotedLocalColumns()),
		utils.ToLowerList(key2.GetUnquotedLocalColumns()),
	)
	if !same {
		return true
	}

	same = slices.Equal(
		utils.ToLowerList(key1.GetUnquotedForeignColumns()),
		utils.ToLowerList(key2.GetUnquotedForeignColumns()),
	)
	if !same {
		return true
	}

	if key1.GetUnqualifiedForeignTableName() != key2.GetUnqualifiedForeignTableName() {
		return true
	}

	if !utils.EqualStringPtr(key1.OnUpdate(), key2.OnUpdate()) {
		return true
	}

	if !utils.EqualStringPtr(key1.OnDelete(), key2.OnDelete()) {
		return true
	}

	return false
}

// columnsEqual - Compares the definitions of the given columns
func (c *Comparator) columnsEqual(column1 *assets.Column, column2 *assets.Column) bool {
	return c.platform.ColumnsEqual(column1, column2)
}

// diffIndex - Finds the difference between the indexes $index1 and $index2.
// Compares index1 with index2 and returns true if there are any
// differences or false in case there are no differences.
func (c *Comparator) diffIndex(index1 *assets.Index, index2 *assets.Index) bool {
	return !(index1.IsFulfilledBy(index2) && index2.IsFulfilledBy(index1))
}
