package dtos

import (
	"github.com/KoNekoD/gormite/pkg/enums"
)

type Union struct {
	Query     *UnionQb
	UnionType *enums.UnionType
}

func NewUnion(query *UnionQb) *Union {
	return &Union{Query: query}
}

func NewUnionWithType(query *UnionQb, unionType enums.UnionType) *Union {
	return &Union{Query: query, UnionType: &unionType}
}
