package dtos

type QueryBuilderInterface interface {
	ToString() string
}

// UnionQb represents a union query builder, it can be a query builder or a string
type UnionQb struct {
	QueryBuilder QueryBuilderInterface
	String       *string
}

func (u *UnionQb) ToString() string {
	if u.String != nil {
		return *u.String
	}
	return u.QueryBuilder.ToString()
}
