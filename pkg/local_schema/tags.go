package local_schema

import (
	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/types"
	"github.com/pkg/errors"
	"slices"
	"strconv"
)

const (
	columnTagName                    = "db"
	primaryKeyTagName                = "pk"
	isNullableTagName                = "nullable"
	lengthTagName                    = "length"
	onUpdateTagName                  = "on_update"
	onDeleteTagName                  = "on_delete"
	uniqueConstraintTagName          = "uniq"
	uniqueConstraintConditionTagName = "uniq_cond"
	indexTagName                     = "index"
	indexConditionTagName            = "index_cond"
	defaultValueTagName              = "default"
	typeTagName                      = "type"
	precisionTagName                 = "precision"
	scaleTagName                     = "scale"
)

func (t *tableBag) parseColumnTags(typeName string, objectsKeys []string) (*columnData, error) {
	tags := t.currentFieldTags

	colNameTag, _ := tags.Get(columnTagName)
	columnName := colNameTag.Value()

	isForeignKey := slices.Contains(objectsKeys, typeName)
	onUpdateTag, _ := tags.Get(onUpdateTagName)
	var onUpdate *string
	if onUpdateTag != nil {
		onUpdateTmp := onUpdateTag.Value()
		onUpdate = &onUpdateTmp
	}
	onDeleteTag, _ := tags.Get(onDeleteTagName)
	var onDelete *string
	if onDeleteTag != nil {
		onDeleteTmp := onDeleteTag.Value()
		onDelete = &onDeleteTmp
	}

	pk, _ := tags.Get(primaryKeyTagName)
	isPrimaryKey := pk != nil

	nullableTag, _ := tags.Get(isNullableTagName)
	isNullable := nullableTag != nil
	isNotNull := !isNullable

	lengthTag, _ := tags.Get(lengthTagName)
	length := 255
	if lengthTag != nil {
		var err error
		length, err = strconv.Atoi(lengthTag.Value())
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse length tag")
		}
	}

	uniqTag, _ := tags.Get(uniqueConstraintTagName)
	isUnique := uniqTag != nil
	var uniqueName *string
	if isUnique {
		uniqTagName := uniqTag.Value()
		uniqueName = &uniqTagName
	}
	uniqCondTag, _ := tags.Get(uniqueConstraintConditionTagName)
	isUniqueCondition := uniqCondTag != nil
	var uniqueCondition *string
	if isUniqueCondition {
		uniqCondTagName := uniqCondTag.Value()
		uniqueCondition = &uniqCondTagName
	}

	indexTag, _ := tags.Get(indexTagName)
	isIndex := indexTag != nil
	var indexName *string
	if isIndex && indexTag != nil {
		indexNameTmp := indexTag.Value()
		indexName = &indexNameTmp
	}
	indexCondTag, _ := tags.Get(indexConditionTagName)
	isIndexCondition := indexCondTag != nil
	var indexCondition *string
	if isIndexCondition && indexCondTag != nil {
		indexConditionTmp := indexCondTag.Value()
		indexCondition = &indexConditionTmp
	}

	var defaultValue *string
	if defaultTag, _ := tags.Get(defaultValueTagName); defaultTag != nil {
		defaultValueTmp := defaultTag.Value()
		defaultValue = &defaultValueTmp
	}

	var columnType types.AbstractTypeInterface
	options := make([]assets.ColumnOption, 0)

	typeTag, _ := tags.Get(typeTagName)
	if typeTag != nil {
		typeTagValue := typeTag.Value()
		switch typeTagValue {
		case "text":
			columnType = types.NewTextType()
		case "varchar":
			columnType = types.NewStringType()
		case "json":
			columnType = types.NewJsonType()
			typeName = "json"
		case "jsonb":
			typeName = "jsonb"
			columnType = types.NewJsonType()
			options = append(options, func(c *assets.Column) { c.SetPlatformOption("jsonb", true) })
		case "integer":
			columnType = types.NewIntegerType()
		case "bigint":
			columnType = types.NewBigintType()
		case "decimal":
			columnType = types.NewDecimalType()
			precisionTag, _ := tags.Get(precisionTagName)
			if precisionTag != nil {
				precisionTagValue, _ := strconv.Atoi(precisionTag.Value())
				options = append(options, func(c *assets.Column) { c.SetPrecision(precisionTagValue) })
			}
			scaleTag, _ := tags.Get(scaleTagName)
			if scaleTag != nil {
				scaleTagValue, _ := strconv.Atoi(scaleTag.Value())
				options = append(options, func(c *assets.Column) { c.SetScale(scaleTagValue) })
			}

		case "float":
			columnType = types.NewFloatType()
		case "smallfloat":
			columnType = types.NewSmallFloatType()
		default:
			return nil, errors.Errorf("unknown type tag value %s", typeTagValue)
		}
	}
	if columnType == nil {
		switch typeName {
		case "int":
			columnType = types.NewIntegerType()
		case "string":
			columnType = types.NewStringType()
		case "bool":
			columnType = types.NewBooleanType()
		case "float32":
			columnType = types.NewSmallFloatType()
		case "float64":
			columnType = types.NewFloatType()
		}
	}

	if isNotNull {
		options = append(options, assets.WithColumnNotNull())
	}
	if defaultValue != nil {
		options = append(options, assets.WithColumnDefault(*defaultValue))
	}
	isColumnTypeVarchar := false
	if columnType != nil {
		_, isColumnTypeVarchar = columnType.(*types.StringType)
	}
	if typeName == "string" || isColumnTypeVarchar {
		options = append(options, assets.WithColumnLength(&length))
	}

	return &columnData{
		ColumnName:        columnName,
		IsPrimaryKey:      isPrimaryKey,
		IsForeignKey:      isForeignKey,
		OnUpdate:          onUpdate,
		OnDelete:          onDelete,
		IsNotNull:         isNotNull,
		TypeName:          typeName,
		IsUnique:          isUnique,
		UniqueName:        uniqueName,
		IsUniqueCondition: isUniqueCondition,
		UniqueCondition:   uniqueCondition,
		IsIndex:           isIndex,
		IndexName:         indexName,
		IsIndexCondition:  isIndexCondition,
		IndexCondition:    indexCondition,
		Length:            length,
		DefaultValue:      defaultValue,
		ColumnType:        columnType,
		HasTypeTag:        typeTag != nil,
		Options:           options,
	}, nil
}
