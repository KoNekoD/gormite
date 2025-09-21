package types

import (
	"fmt"
	"github.com/KoNekoD/gormite/pkg/enums"
)

// BuiltinTypesMap - The map of supported doctrine mapping types.
var BuiltinTypesMap = map[enums.TypesType]AbstractTypeInterface{
	enums.TypeAsciiString:         &AsciiStringType{StringType: NewStringType()},
	enums.TypeBigint:              NewBigintType(),
	enums.TypeBoolean:             NewBooleanType(),
	enums.TypeDecimal:             NewDecimalType(),
	enums.TypeFloat:               NewFloatType(),
	enums.TypeSmallfloat:          NewSmallFloatType(),
	enums.TypeInteger:             NewIntegerType(),
	enums.TypeJson:                NewJsonType(),
	enums.TypeString:              NewStringType(),
	enums.TypeText:                NewTextType(),
	enums.TypeBinary:              &BinaryType{AbstractType: &AbstractType{}},
	enums.TypeBlob:                &BlobType{AbstractType: &AbstractType{}},
	enums.TypeDateMutable:         &DateType{AbstractType: &AbstractType{}},
	enums.TypeDateImmutable:       &DateImmutableType{AbstractType: &AbstractType{}},
	enums.TypeDateinterval:        &DateIntervalType{AbstractType: &AbstractType{}},
	enums.TypeDatetimeMutable:     &DateTimeType{AbstractType: &AbstractType{}},
	enums.TypeDatetimeImmutable:   NewDateTimeImmutableType(),
	enums.TypeDatetimetzMutable:   &DateTimeTzType{AbstractType: &AbstractType{}},
	enums.TypeDatetimetzImmutable: &DateTimeTzImmutableType{AbstractType: &AbstractType{}},
	enums.TypeGuid:                &GuidType{StringType: NewStringType()},
	enums.TypeSimpleArray:         &SimpleArrayType{AbstractType: &AbstractType{}},
	enums.TypeSmallint:            &SmallIntType{AbstractType: &AbstractType{}},
	enums.TypeTimeMutable:         &TimeType{AbstractType: &AbstractType{}},
	enums.TypeTimeImmutable:       &TimeImmutableType{AbstractType: &AbstractType{}},
}

type AbstractTypeInterface interface {
	GetSQLDeclaration(column map[string]any, platform TypesPlatform) string
	ConvertToPHPValue(value any, platform TypesPlatform) any
	GetMappedDatabaseTypes(platform TypesPlatform) []string
}

var typeRegistry *TypeRegistry

type AbstractType struct{}

func (a *AbstractType) NewAbstractType() *AbstractType {
	return &AbstractType{}
}
func (a *AbstractType) ConvertToDatabaseValue(value any, platform TypesPlatform) any {
	return value
}
func (a *AbstractType) ConvertToPHPValue(value any, platform TypesPlatform) any {
	return value
}
func GetTypeRegistry() *TypeRegistry {
	if typeRegistry != nil {
		return typeRegistry
	}

	return createTypeRegistry()
}
func createTypeRegistry() *TypeRegistry {
	instances := make(map[enums.TypesType]AbstractTypeInterface)

	for name, class := range BuiltinTypesMap {
		instances[name] = class
	}

	return &TypeRegistry{instances: instances}
}
func GetType(name enums.TypesType) AbstractTypeInterface {
	registry := GetTypeRegistry()
	return registry.Get(name)
}
func LookupName(typeVar AbstractTypeInterface) enums.TypesType {
	registry := GetTypeRegistry()
	return registry.LookupName(typeVar)
}
func AddType(name enums.TypesType, className AbstractTypeInterface) {
	registry := GetTypeRegistry()
	registry.Register(name, className)
}
func HasType(name enums.TypesType) bool {
	registry := GetTypeRegistry()
	return registry.Has(name)
}
func OverrideType(name enums.TypesType, className AbstractTypeInterface) {
	registry := GetTypeRegistry()
	registry.Override(name, className)
}
func (a *AbstractType) GetBindingType() enums.ParameterType {
	return enums.ParameterTypeString
}

func GetTypesMap() map[enums.TypesType]string {
	result := make(map[enums.TypesType]string)

	for typesType, typeInterface := range GetTypeRegistry().GetMap() {
		result[typesType] = fmt.Sprintf("%T", typeInterface)
	}

	return result
}

func (a *AbstractType) ConvertToDatabaseValueSQL(sqlExpr string, platform TypesPlatform) string {
	return sqlExpr
}

func (a *AbstractType) ConvertToPHPValueSQL(sqlExpr string, platform TypesPlatform) string {
	return sqlExpr
}

func (a *AbstractType) GetMappedDatabaseTypes(platform TypesPlatform) []string {
	return make([]string, 0)
}
