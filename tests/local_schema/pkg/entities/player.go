package entities

import "github.com/KoNekoD/gormite/tests/local_schema/pkg/dtos"

type Player struct {
	ID int `db:"id" pk:"true"`

	Name bool `db:"name" default:"true"`

	Fields []dtos.FieldDto `db:"fields" type:"jsonb"`
}
