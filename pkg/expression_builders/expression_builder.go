package expression_builders

import (
	"github.com/KoNekoD/gormite/pkg/dtos"
	"strings"
)

// ExpressionBuilder structure is responsible to dynamically create SQL query parts.
type ExpressionBuilder struct{}

const (
	EQ  = "="
	NEQ = "<>"
	LT  = "<"
	LTE = "<="
	GT  = ">"
	GTE = ">="
)

// And - Creates a conjunction of the given expressions.
func (b *ExpressionBuilder) And(expr ...string) *dtos.CompositeExpression {
	expressionsComposite := make([]*dtos.Expr, len(expr))
	for i, e := range expr {
		expressionsComposite[i] = &dtos.Expr{Str: &e}
	}

	return dtos.NewAndCompositeExpression(expressionsComposite...)
}

// AndComposite - Creates a conjunction of the given expressions.
func (b *ExpressionBuilder) AndComposite(expr ...*dtos.Expr) *dtos.CompositeExpression {
	return dtos.NewAndCompositeExpression(expr...)
}

// Or - Creates a disjunction of the given expressions.
func (b *ExpressionBuilder) Or(expr ...string) *dtos.CompositeExpression {
	expressionsComposite := make([]*dtos.Expr, len(expr))
	for i, e := range expr {
		expressionsComposite[i] = &dtos.Expr{Str: &e}
	}

	return dtos.NewOrCompositeExpression(expressionsComposite...)
}

func (b *ExpressionBuilder) OrComposite(expr ...*dtos.Expr) *dtos.CompositeExpression {
	return dtos.NewOrCompositeExpression(expr...)
}

// Comparison - Creates a comparison expression.
//
// x - The left expression.
//
// operator - The comparison operator.
//
// y - The right expression.
func (b *ExpressionBuilder) Comparison(x, operator, y string) string {
	return x + " " + operator + " " + y
}

// Eq - Creates an equality comparison expression with the given arguments.
//
// First argument is considered the left expression and the second is the right expression.
// When converted to string, it will be generated a <left expr> = <right expr>. Example:
//
//	[go]
//	// u.id = ?
//	expr.eq("u.id", "?");
//
// x - The left expression.
//
// y - The right expression.
func (b *ExpressionBuilder) Eq(x, y string) string {
	return b.Comparison(x, EQ, y)
}

// Neq - Creates a non equality comparison expression with the given arguments.
// First argument is considered the left expression and the second is the right expression.
// When converted to string, it will be generated a <left expr> <> <right expr>. Example:
//
//	[go]
//	// u.id <> 1
//	q.where(q.expr().neq("u.id", "1"));
//
// x - The left expression.
//
// y - The right expression.
func (b *ExpressionBuilder) Neq(x, y string) string {
	return b.Comparison(x, NEQ, y)
}

// Lt - Creates a lower-than comparison expression with the given arguments.
// First argument is considered the left expression and the second is the right expression.
// When converted to string, it will be generated a <left expr> < <right expr>. Example:
//
//	[go]
//	// u.id > ?
//	q.where(q.expr().lt("u.id", "?"));
//
// x - The left expression.
//
// y - The right expression.
func (b *ExpressionBuilder) Lt(x, y string) string {
	return b.Comparison(x, LT, y)
}

// Lte - Creates a lower-than-equal comparison expression with the given arguments.
// First argument is considered the left expression and the second is the right expression.
// When converted to string, it will be generated a <left expr> <= <right expr>. Example:
//
//	[go]
//	// u.id <= ?
//	q.where(q.expr().lte("u.id", "?"));
//
// x - The left expression.
// y - The right expression.
func (b *ExpressionBuilder) Lte(x, y string) string {
	return b.Comparison(x, LTE, y)
}

// Gt - Creates a greater-than comparison expression with the given arguments.
// First argument is considered the left expression and the second is the right expression.
// When converted to string, it will be generated a <left expr> > <right expr>. Example:
//
//	[go]
//	// u.id > ?
//	q.where(q.expr().gt("u.id", "?"));
//
// x - The left expression.
//
// y - The right expression.
func (b *ExpressionBuilder) Gt(x, y string) string {
	return b.Comparison(x, GT, y)
}

// Gte - Creates a greater-than-equal comparison expression with the given arguments.
// First argument is considered the left expression and the second is the right expression.
// When converted to string, it will be generated a <left expr> >= <right expr>. Example:
//
//	[go]
//	// u.id >= ?
//	q.where(q.expr().gte("u.id", "?"));
//
// x - The left expression.
// y - The right expression.
func (b *ExpressionBuilder) Gte(x, y string) string {
	return b.Comparison(x, GTE, y)
}

// IsNull - Creates an IS NULL expression with the given arguments.
//
// x - The expression to be restricted by IS NULL.
func (b *ExpressionBuilder) IsNull(x string) string {
	return x + " IS NULL"
}

// IsNotNull - Creates an IS NOT NULL expression with the given arguments.
//
// x - The expression to be restricted by IS NOT NULL.
func (b *ExpressionBuilder) IsNotNull(x string) string {
	return x + " IS NOT NULL"
}

// Like - Creates a LIKE comparison expression.
//
// expression - The expression to be inspected by the LIKE comparison.
// pattern - The pattern to compare against.
func (b *ExpressionBuilder) Like(expression, pattern string) string {
	return b.Comparison(expression, "LIKE", pattern)
}

// NotLike - Creates a NOT LIKE comparison expression.
//
// expression - The expression to be inspected by the NOT LIKE comparison.
// pattern - The pattern to compare against.
func (b *ExpressionBuilder) NotLike(expression, pattern string) string {
	return b.Comparison(expression, "NOT LIKE", pattern)
}

// LikeWithEscapeChar - Creates a LIKE comparison expression.
//
// expression - The expression to be inspected by the LIKE comparison.
// pattern - The pattern to compare against.
// escapeChar - Optional escape character for special characters.
func (b *ExpressionBuilder) LikeWithEscapeChar(expression, pattern, escapeChar string) string {
	result := b.Comparison(expression, "LIKE", pattern)
	if escapeChar != "" {
		result += " ESCAPE " + escapeChar
	}
	return result
}

// NotLikeWithEscapeChar - Creates a NOT LIKE comparison expression.
//
// expression - The expression to be inspected by the NOT LIKE comparison.
// pattern - The pattern to compare against.
// escapeChar - Optional escape character for special characters.
func (b *ExpressionBuilder) NotLikeWithEscapeChar(expression, pattern, escapeChar string) string {
	result := b.Comparison(expression, "NOT LIKE", pattern)
	if escapeChar != "" {
		result += " ESCAPE " + escapeChar
	}
	return result
}

// In - Creates an IN () comparison expression with the given arguments.
//
// x - The SQL expression to be matched against the set.
//
// y - The SQL expression or an array of SQL expressions representing the set.
func (b *ExpressionBuilder) In(x string, y []string) string {
	return b.Comparison(x, "IN", "("+strings.Join(y, ", ")+")")
}

// NotIn - Creates a NOT IN () comparison expression with the given arguments.
//
// x - The SQL expression to be matched against the set.
//
// y - The SQL expression or an array of SQL expressions representing the set.
func (b *ExpressionBuilder) NotIn(x string, y []string) string {
	return b.Comparison(x, "NOT IN", "("+strings.Join(y, ", ")+")")
}

// Exists - Creates an EXISTS (subquery) expression.
//
// subquery - The SQL subquery to be checked for existence.
func (b *ExpressionBuilder) Exists(subquery string) string {
	return "EXISTS (" + subquery + ")"
}

// NotExists - Creates a NOT EXISTS (subquery) expression.
//
// subquery - The SQL subquery to be checked for non-existence.
func (b *ExpressionBuilder) NotExists(subquery string) string {
	return "NOT EXISTS (" + subquery + ")"
}
