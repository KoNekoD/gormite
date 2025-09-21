package local_schema

import (
	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/types"
	"github.com/fatih/structtag"
	"github.com/pkg/errors"
	"go/ast"
	"golang.org/x/exp/maps"
	"slices"
	"strings"
)

func (s *store) newTable(name string) *assets.Table {
	name = getName(s, name)

	for _, table := range s.tables {
		if table.GetName() == name {
			return table
		}
	}

	t := assets.NewTable(name, nil, nil, nil, nil, nil)

	s.tables = append(s.tables, t)

	return t
}

type tableBag struct {
	store       *store
	table       *assets.Table
	primaryKeys []string

	uniqColumnsMap    map[string][]string
	uniqConditionsMap map[string]string

	indexColumnsMap    map[string][]string
	indexConditionsMap map[string]string

	// Current column name level
	currentFieldName string
	currentFieldTags *structtag.Tags
}

func newTableBag(store *store, table *assets.Table) *tableBag {
	bag := &tableBag{
		store:              store,
		table:              table,
		primaryKeys:        make([]string, 0),
		uniqColumnsMap:     make(map[string][]string),
		uniqConditionsMap:  make(map[string]string),
		indexColumnsMap:    make(map[string][]string),
		indexConditionsMap: make(map[string]string),
	}

	return bag
}

func (t *tableBag) colIdent(fieldType *ast.Ident, nillable bool) error {
	objectsKeys := maps.Keys(t.store.objectsMap)

	columnTagsData, err := t.parseColumnTags(fieldType, objectsKeys)
	if err != nil {
		return errors.Wrap(err, "failed to parse column tags")
	}

	if !columnTagsData.IsForeignKey && !columnTagsData.IsNotNull != nillable {
		return errors.Errorf("column %s nullable mismatch", columnTagsData.ColumnName)
	}
	if columnTagsData.IsForeignKey && !nillable {
		return errors.Errorf("column %s must be nullable", columnTagsData.ColumnName)
	}

	if columnTagsData.ColumnType != nil {
		t.table.AddColumn(columnTagsData.ColumnName, columnTagsData.ColumnType, columnTagsData.Options...)
	} else {
		if !columnTagsData.IsForeignKey {
			return errors.Errorf("unknown type %s", columnTagsData.TypeName)
		}

		// Maybe we need rewrite it to allow use non integer ids...
		t.table.AddColumn(columnTagsData.ColumnName, types.NewIntegerType(), columnTagsData.Options...)
	}

	return t.finalizeColumn(columnTagsData)
}

func (t *tableBag) finalizeColumn(columnTagsData *columnData) error {
	err := applyMetadataMutatorsForNewColumn(columnTagsData, t)
	if err != nil {
		return errors.Wrapf(err, "finalize column %s failed", columnTagsData.ColumnName)
	}

	return nil
}

func (t *tableBag) colSel(fType *ast.SelectorExpr, nillable bool) error {
	objectsKeys := maps.Keys(t.store.objectsMap)

	selPackage := fType.X.(*ast.Ident).Name
	selType := fType.Sel.Name

	columnTagsData, err := t.parseColumnTags(fType.Sel, objectsKeys)
	if err != nil {
		return errors.Wrap(err, "failed to parse column tags")
	}

	if selPackage == "time" && selType == "Time" {
		t.table.AddColumn(columnTagsData.ColumnName, types.NewDateTimeImmutableType(), columnTagsData.Options...)
		return t.finalizeColumn(columnTagsData)
	}

	if columnTagsData.ColumnType != nil {
		t.table.AddColumn(columnTagsData.ColumnName, columnTagsData.ColumnType, columnTagsData.Options...)
		return t.finalizeColumn(columnTagsData)
	}

	if found, ok := t.store.structNamesIdentsMap[selType]; ok {
		return t.colIdent(found, nillable)
	}

	return errors.Errorf("unknown type for %s", columnTagsData.ColumnName)
}

func (t *tableBag) colStar(fType *ast.StarExpr) error {
	switch fieldTypeX := fType.X.(type) {
	case *ast.Ident:
		return t.colIdent(fieldTypeX, true)
	case *ast.SelectorExpr:
		return t.colSel(fieldTypeX, true)
	default:
		return errors.Errorf("unknown star type %T", fieldTypeX)
	}
}

func (t *tableBag) colArray(fieldType *ast.ArrayType) error {
	objectsKeys := maps.Keys(t.store.objectsMap)

	ident, ok := fieldType.Elt.(*ast.Ident)
	if !ok {
		return errors.Errorf("only literal array types are supported")
	}

	// TODO: Refactor for others literals support

	allowedFieldTypes := []string{"string"}

	if !slices.Contains(allowedFieldTypes, ident.Name) {
		return errors.Errorf("slice type only allowed for %s", allowedFieldTypes)
	}

	columnTagsData, err := t.parseColumnTags(ident, objectsKeys)
	if err != nil {
		return errors.Wrap(err, "failed to parse column tags")
	}

	allowedTypes := []string{"json", "jsonb"}

	if !columnTagsData.HasTypeTag {
		return errors.Errorf("type tag is required for %s, allowed: %s", columnTagsData.ColumnName, allowedTypes)
	}

	if !slices.Contains(allowedTypes, columnTagsData.TypeName) {
		return errors.Errorf("type tag only allowed for %s", allowedTypes)
	}

	t.table.AddColumn(columnTagsData.ColumnName, columnTagsData.ColumnType, columnTagsData.Options...)

	return nil
}

func handleMappingObject(objectName string, store *store) error {
	t := store.newTable(objectName)

	object := store.objectsMap[objectName]

	typeSpec := object.Decl.(*ast.TypeSpec)
	structType := typeSpec.Type.(*ast.StructType)

	bag := newTableBag(store, t)

	for _, field := range structType.Fields.List {
		if len(field.Names) != 1 {
			return errors.New("only single field names are supported")
		}

		var tags *structtag.Tags
		var err error

		if field.Tag != nil {
			tag := strings.Trim(field.Tag.Value, "`")
			tags, err = structtag.Parse(tag)
			if err != nil {
				return errors.Wrap(err, "failed to parse tags")
			}
		}
		bag.currentFieldName = field.Names[0].Name
		bag.currentFieldTags = tags

		switch fType := field.Type.(type) {
		case *ast.Ident:
			err = bag.colIdent(fType, false)
		case *ast.StarExpr:
			err = bag.colStar(fType)
		case *ast.SelectorExpr:
			err = bag.colSel(fType, false)
		case *ast.ArrayType:
			err = bag.colArray(fType)
		default:
			return errors.Errorf("unknown type %T of table %s", fType, bag.table.GetName())
		}

		if err != nil {
			return errors.Wrapf(err, "failed to handle field %s of table %s", field.Names[0].Name, bag.table.GetName())
		}
	}

	err := applyMetadataMutatorsAfterColumnsIntrospection(bag)
	if err != nil {
		return errors.Wrapf(err, "failed to finalize table %s", bag.table.GetName())
	}

	return nil
}
