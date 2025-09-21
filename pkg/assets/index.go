package assets

import (
	"github.com/KoNekoD/gormite/pkg/utils"
	"golang.org/x/exp/maps"
	"slices"
	"strings"
)

type Index struct {
	*AbstractAsset

	// columns Asset identifier instances of the column names
	// the index is associated with
	columns map[string]*Identifier

	isPrimary bool

	isUnique bool

	// flags Platform specific flags for indexes
	flags map[string]bool

	options map[string]any
}

func NewIndex(
	name string,
	columns []string,
	isUnique bool,
	isPrimary bool,
	flags []string,
	options map[string]any,
) *Index {
	isUnique = isUnique || isPrimary

	v := &Index{
		AbstractAsset: NewAbstractAsset(),
		columns:       make(map[string]*Identifier),
		flags:         make(map[string]bool),
		options:       options,
	}

	if name != "" {
		v.SetName(name)
	}

	v.isUnique = isUnique
	v.isPrimary = isPrimary

	for _, column := range columns {
		v.addColumn(column)
	}

	for _, flag := range flags {
		v.AddFlag(flag)
	}

	return v
}

func (i *Index) addColumn(column string) {
	i.columns[column] = NewIdentifier(column)
}

// GetColumns Returns the names of the referencing table columns the
// constraint is associated with
func (i *Index) GetColumns() []string {
	return maps.Keys(i.columns)
}

func (i *Index) GetColumn(key string) *Identifier {
	v, ok := i.columns[key]

	if !ok {
		return nil
	}

	return v
}

// GetQuotedColumns Returns the quoted representation of the
// column names the constraint is associated with
// But only if they were defined with one or a column name
// is a keyword reserved by the platform
// Otherwise, the plain unquoted value as inserted is returned
func (i *Index) GetQuotedColumns(platform AssetsPlatform) []string {
	subParts := make([]string, 0)

	if platform.SupportsColumnLengthIndexes() && i.HasOption("lengths") {
		subParts = i.GetOption("lengths").([]string)
	}

	columns := make([]string, 0)

	for _, column := range i.columns {
		quotedColumn := column.GetQuotedName(platform)

		if len(subParts) > 0 {
			length := subParts[0]

			quotedColumn += "(" + length + ")"

			subParts = subParts[1:]
		}

		columns = append(columns, quotedColumn)
	}

	return columns
}

func (i *Index) GetUnquotedColumns() []string {
	columns := make([]string, 0, len(i.columns))

	for _, column := range i.GetColumns() {
		columns = append(columns, i.trimQuotes(column))
	}

	return columns
}

// IsSimpleIndex Is the index neither unique nor primary key?
func (i *Index) IsSimpleIndex() bool {
	return !i.IsPrimary() && !i.IsUnique()
}

func (i *Index) IsPrimary() bool {
	return i.isPrimary
}

func (i *Index) IsUnique() bool {
	return i.isUnique
}

func (i *Index) HasColumnAtPosition(name string, pos int) bool {
	name = i.trimQuotes(strings.ToLower(name))

	for index, s := range i.GetUnquotedColumns() {
		s = strings.ToLower(s)

		if s == name {
			return index == pos
		}
	}

	return false
}

// SpansColumns Checks if this index exactly spans the given
// column names in the correct order
func (i *Index) SpansColumns(columnNames []string) bool {
	columns := i.GetColumns()
	numberOfColumns := len(columns)
	sameColumns := true

	slices.Sort(columns)
	slices.Sort(columnNames)

	for j := 0; j < numberOfColumns; j++ {
		if len(columnNames) > j {
			indexColumn := i.trimQuotes(strings.ToLower(columns[j]))
			inputColumn := i.trimQuotes(strings.ToLower(columnNames[j]))

			if indexColumn == inputColumn {
				continue
			}
		}

		sameColumns = false
	}

	return sameColumns
}

// IsFulfilledBy Checks if the other index already fulfills all the
// indexing and constraint needs of the current one
func (i *Index) IsFulfilledBy(other *Index) bool {
	// allow the other index to be equally large only
	// It being larger is an option, but it creates a problem with
	// scenarios of the kind PRIMARY KEY(foo,bar) UNIQUE(foo)
	if len(other.GetColumns()) != len(i.GetColumns()) {
		return false
	}

	// Check if columns are the same, and even in the same order
	sameColumns := i.SpansColumns(other.GetColumns())

	if sameColumns {
		if !i.samePartialIndex(other) {
			return false
		}

		if !i.hasSameColumnLengths(other) {
			return false
		}

		if !i.IsUnique() && !i.IsPrimary() {
			// this is a special case: If the current key is neither primary or unique, any unique or
			// primary key will always have the same effect for the index and there cannot be any constraint
			// overlaps. This means a primary or unique index can always fulfill the requirements of just an
			// index that has no constraints.
			return true
		}

		if other.IsPrimary() != i.IsPrimary() {
			return false
		}

		return other.IsUnique() == i.IsUnique()
	}

	return false
}

// overrules Detects if the other index is a non-unique, non-primary
// index that can be overwritten by this one
func (i *Index) overrules(other *Index) bool {
	if other.IsPrimary() {
		return false
	}

	if i.IsSimpleIndex() && other.IsUnique() {
		return false
	}

	return i.SpansColumns(other.GetColumns()) &&
		(i.IsPrimary() || i.IsUnique()) &&
		i.samePartialIndex(other)
}

// GetFlags Returns platform specific flags for indexes
func (i *Index) GetFlags() []string {
	return maps.Keys(i.flags)
}

// AddFlag Adds Flag for an index that translates to platform specific handling
func (i *Index) AddFlag(flag string) *Index {
	i.flags[strings.ToLower(flag)] = true

	return i
}

// HasFlag Does this index have a specific flag?
func (i *Index) HasFlag(flag string) bool {
	_, ok := i.flags[strings.ToLower(flag)]

	return ok
}

// RemoveFlag Removes a flag.
func (i *Index) RemoveFlag(flag string) {
	delete(i.flags, strings.ToLower(flag))
}

func (i *Index) HasOption(name string) bool {
	_, ok := i.options[name]

	return ok
}

func (i *Index) GetOption(name string) any {
	v, ok := i.options[strings.ToLower(name)]

	if !ok {
		panic("option " + name + " not found")
	}

	return v
}

func (i *Index) GetOptions() map[string]any {
	return i.options
}

// samePartialIndex Return whether the two indexes have the same partial index
func (i *Index) samePartialIndex(other *Index) bool {
	if i.HasOption("where") && other.HasOption("where") &&
		i.GetOption("where") == other.GetOption("where") {
		return true
	}

	return !i.HasOption("where") && !other.HasOption("where")
}

// hasSameColumnLengths Returns whether the index has the
// same column lengths as the other
func (i *Index) hasSameColumnLengths(other *Index) bool {
	iLengthVal, ok1 := i.options["lengths"]
	otherLengthVal, ok2 := other.options["lengths"]

	if !ok1 || !ok2 {
		return ok1 == ok2 // If one ok two not provided, check if two is not provided, otherwise false
	}

	same := slices.Equal(utils.FilterIntsPtr(iLengthVal.([]*int)), utils.FilterIntsPtr(otherLengthVal.([]*int)))

	return same
}
