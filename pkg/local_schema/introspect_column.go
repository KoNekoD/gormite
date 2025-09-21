package local_schema

import (
	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/types"
)

type columnData struct {
	ColumnName   string
	IsPrimaryKey bool
	IsNotNull    bool

	IsForeignKey bool
	OnDelete     *string
	OnUpdate     *string

	TypeName string

	IsUnique          bool
	UniqueName        *string
	IsUniqueCondition bool
	UniqueCondition   *string

	IsIndex          bool
	IndexName        *string
	IsIndexCondition bool
	IndexCondition   *string

	Length       int
	DefaultValue *string

	ColumnType types.AbstractTypeInterface
	HasTypeTag bool

	Options []assets.ColumnOption
}
