package assets

import (
	"github.com/KoNekoD/gormite/pkg/utils"
	"golang.org/x/exp/maps"
	"strings"
)

// ForeignKeyConstraint An abstraction class for a foreign key constraint
type ForeignKeyConstraint struct {
	*AbstractAsset

	// localColumnNames Asset identifier instances of the referencing table
	// column names the foreign key constraint is associated with
	//
	// Names of the referencing table columns
	localColumnNames map[string]*Identifier

	// foreignTableName Table or asset identifier instance of the
	// referenced table name the foreign key constraint is associated with
	//
	// Referenced table
	foreignTableName *Identifier

	// foreignColumnNames Asset identifier instances of the referenced table
	// column names the foreign key constraint is associated with
	//
	// Names of the referenced table columns
	foreignColumnNames map[string]*Identifier

	options map[string]any
}

// NewForeignKeyConstraint Initializes the foreign key constraint
func NewForeignKeyConstraint(
	name string, // Name of the foreign key constraint
	localColumnNames []string, // Referenced table
	foreignTableName string, // Names of the referenced table columns
	foreignColumnNames []string, // Name of the foreign key constraint
	options map[string]any, // Options for the foreign key constraint
) *ForeignKeyConstraint {
	v := &ForeignKeyConstraint{AbstractAsset: NewAbstractAsset()}

	v.SetName(name)

	v.localColumnNames = v.createIdentifierMap(localColumnNames)
	v.foreignTableName = NewIdentifier(foreignTableName)
	v.foreignColumnNames = v.createIdentifierMap(foreignColumnNames)

	v.options = options

	return v
}

func (c *ForeignKeyConstraint) createIdentifierMap(names []string) map[string]*Identifier {
	identifiers := make(map[string]*Identifier)

	for _, name := range names {
		identifiers[name] = NewIdentifier(name)
	}

	return identifiers
}

// GetLocalColumns Returns the names of the referencing table columns
// the foreign key constraint is associated with
func (c *ForeignKeyConstraint) GetLocalColumns() []string {
	return maps.Keys(c.localColumnNames)
}

// GetQuotedLocalColumns Returns the quoted representation of the referencing
// table column names the foreign key constraint is associated with
//
// But only if they were defined with one or the referencing table column name
// is a keyword reserved by the platform
// Otherwise, the plain unquoted value as inserted is returned
func (c *ForeignKeyConstraint) GetQuotedLocalColumns(platform AssetsPlatform) []string {
	columns := make([]string, 0, len(c.localColumnNames))

	for _, column := range c.localColumnNames {
		columns = append(columns, column.GetQuotedName(platform))
	}

	return columns
}

// GetUnquotedLocalColumns returns unquoted representation of local table column names for comparison with other FK
func (c *ForeignKeyConstraint) GetUnquotedLocalColumns() []string {
	items := c.GetLocalColumns()

	result := make([]string, len(items))

	for i, item := range items {
		result[i] = c.trimQuotes(item)
	}

	return result
}

// GetUnquotedForeignColumns returns unquoted representation of foreign table column names for comparison with other FK
func (c *ForeignKeyConstraint) GetUnquotedForeignColumns() []string {
	items := c.GetForeignColumns()

	result := make([]string, len(items))

	for i, item := range items {
		result[i] = c.trimQuotes(item)
	}

	return result
}

// GetForeignTableName Returns the name of the referenced table the foreign key constraint is associated with
func (c *ForeignKeyConstraint) GetForeignTableName() string {
	return c.foreignTableName.GetName()
}

// GetUnqualifiedForeignTableName Returns the non-schema qualified foreign table name
func (c *ForeignKeyConstraint) GetUnqualifiedForeignTableName() string {
	name := c.foreignTableName.GetName()
	position := strings.Index(name, ".")

	if position != -1 {
		name = utils.Substr(name, position+1, 0)
	}

	return strings.ToLower(name)
}

// GetQuotedForeignTableName Returns the quoted representation of
// the referenced table name the foreign key constraint is associated with
//
// But only if it was defined with one or the referenced table name
// is a keyword reserved by the platform
// Otherwise, the plain unquoted value as inserted is returned
func (c *ForeignKeyConstraint) GetQuotedForeignTableName(platform AssetsPlatform) string {
	return c.foreignTableName.GetQuotedName(platform)
}

// GetForeignColumns Returns the names of the referenced table columns the foreign key constraint is associated with
func (c *ForeignKeyConstraint) GetForeignColumns() []string {
	return maps.Keys(c.foreignColumnNames)
}

// GetQuotedForeignColumns Returns the quoted representation of the referenced
// table column names the foreign key constraint is associated with
//
// But only if they were defined with one or the referenced table column name
// is a keyword reserved by the platform
// Otherwise, the plain unquoted value as inserted is returned
func (c *ForeignKeyConstraint) GetQuotedForeignColumns(platform AssetsPlatform) []string {
	columns := make([]string, 0, len(c.foreignColumnNames))

	for _, column := range c.foreignColumnNames {
		columns = append(columns, column.GetQuotedName(platform))
	}

	return columns
}

// HasOption Returns whether a given option
// is associated with the foreign key constraint
func (c *ForeignKeyConstraint) HasOption(name string) bool {
	_, ok := c.options[name]

	return ok
}

// GetOption Returns an option associated with the foreign key constraint
func (c *ForeignKeyConstraint) GetOption(name string) any {
	v, ok := c.options[name]

	if !ok {
		panic("option " + name + " not found")
	}

	return v
}

// GetOptions Returns the options associated with the foreign key constraint
func (c *ForeignKeyConstraint) GetOptions() map[string]any {
	return c.options
}

// OnUpdate Returns the referential action for UPDATE operations
// on the referenced table the foreign key constraint is associated with
func (c *ForeignKeyConstraint) OnUpdate() *string {
	return c.onEvent("onUpdate")
}

// OnDelete Returns the referential action for DELETE operations
// on the referenced table the foreign key constraint is associated with
func (c *ForeignKeyConstraint) OnDelete() *string {
	return c.onEvent("onDelete")
}

// onEvent Returns the referential action for a given database operation
// on the referenced table the foreign key constraint is associated with
func (c *ForeignKeyConstraint) onEvent(name string) *string {
	v, ok := c.options[name]

	if !ok {
		return nil
	}

	if v != "NO ACTION" && v != "RESTRICT" {
		return v.(*string)
	}

	return nil
}

// IntersectsIndexColumns Checks whether this foreign key constraint
// intersects the given index columns
// Returns `true` if at least one of this foreign key's local columns
// matches one of the given index's columns, `false` otherwise
func (c *ForeignKeyConstraint) IntersectsIndexColumns(index *Index) bool {
	for _, indexColumn := range index.GetColumns() {
		for _, localColumn := range c.localColumnNames {
			indexColumnName := strings.ToLower(indexColumn)
			localColumnName := strings.ToLower(localColumn.GetName())

			if indexColumnName == localColumnName {
				return true
			}
		}
	}

	return false
}
