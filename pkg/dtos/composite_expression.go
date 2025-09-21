package dtos

import "strings"

type ExprType string

const (
	// CompositeExpressionTypeAnd - Constant that represents an AND composite expression.
	CompositeExpressionTypeAnd ExprType = "AND"

	// CompositeExpressionTypeOr - Constant that represents an OR composite expression.
	CompositeExpressionTypeOr ExprType = "OR"
)

// Expr expression is responsible to build a single expression, it can be a string or a composite expression
type Expr struct {
	Expr *CompositeExpression
	Str  *string
}

func (c *Expr) ToString() string {
	if c.Expr != nil {
		return c.Expr.ToString()
	}

	return *c.Str
}

func (c *Expr) Clone() *Expr {
	cloned := *c

	if c.Str != nil {
		str := *c.Str
		cloned.Str = &str
	}

	if c.Expr != nil {
		cloned.Expr = c.Expr.Clone()
	}

	return &cloned
}

// CompositeExpression composite expression is responsible to build a group of similar expression
type CompositeExpression struct {
	exprType string
	// parts - Each expression part of the composite expression.
	parts []*Expr
}

// NewCompositeExpression - Use the NewAndCompositeExpression() / NewOrCompositeExpression() factory methods.
func NewCompositeExpression(
	expressionType ExprType,
	parts ...*Expr,
) *CompositeExpression {
	return &CompositeExpression{exprType: string(expressionType), parts: parts}
}

func NewAndCompositeExpression(parts ...*Expr) *CompositeExpression {
	return NewCompositeExpression(CompositeExpressionTypeAnd, parts...)
}

func NewOrCompositeExpression(parts ...*Expr) *CompositeExpression {
	return NewCompositeExpression(CompositeExpressionTypeOr, parts...)
}

// With - Returns a new CompositeExpression with the given parts added.
func (c *CompositeExpression) With(parts ...*Expr) *CompositeExpression {
	that := c.Clone()

	that.parts = append(that.parts, parts...)

	return that
}

// Count - Retrieves the amount of expressions on composite expression.
func (c *CompositeExpression) Count() int {
	return len(c.parts)
}

// ToString - Retrieves the string representation of this composite expression.
func (c *CompositeExpression) ToString() string {
	if c.Count() == 1 {
		return c.parts[0].ToString()
	}

	partsStrings := make([]string, c.Count())
	for i, part := range c.parts {
		partsStrings[i] = part.ToString()
	}

	return "(" + strings.Join(partsStrings, ") "+c.exprType+" (") + ")"
}

// GetType - Returns the type of this composite expression (AND/OR).
func (c *CompositeExpression) GetType() string {
	return c.exprType
}

func (c *CompositeExpression) Clone() *CompositeExpression {
	cloned := *c

	cloned.parts = make([]*Expr, len(c.parts))
	for i, part := range c.parts {
		cloned.parts[i] = part.Clone()
	}

	return &cloned
}
