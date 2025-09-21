package dtos

import "strings"

type CompositeExpressionType string

const (
	// CompositeExpressionTypeAnd - Constant that represents an AND composite expression.
	CompositeExpressionTypeAnd CompositeExpressionType = "AND"

	// CompositeExpressionTypeOr - Constant that represents an OR composite expression.
	CompositeExpressionTypeOr CompositeExpressionType = "OR"
)

type CompositeExpressionOrString struct {
	CompositeExpression *CompositeExpression
	String              *string
}

func (c *CompositeExpressionOrString) ToString() string {
	if c.CompositeExpression != nil {
		return c.CompositeExpression.ToString()
	}

	return *c.String
}

func (c *CompositeExpressionOrString) Clone() *CompositeExpressionOrString {
	cloned := *c

	if c.String != nil {
		str := *c.String
		cloned.String = &str
	}

	if c.CompositeExpression != nil {
		cloned.CompositeExpression = c.CompositeExpression.Clone()
	}

	return &cloned
}

// CompositeExpression - Composite expression is responsible to build a group of similar expression.
// This class is immutable.
type CompositeExpression struct {
	exprType string
	// parts - Each expression part of the composite expression.
	parts []*CompositeExpressionOrString
}

// NewCompositeExpression - Use the NewAndCompositeExpression() / NewOrCompositeExpression() factory methods.
func NewCompositeExpression(
	expressionType CompositeExpressionType,
	parts ...*CompositeExpressionOrString,
) *CompositeExpression {
	return &CompositeExpression{exprType: string(expressionType), parts: parts}
}

func NewAndCompositeExpression(parts ...*CompositeExpressionOrString) *CompositeExpression {
	return NewCompositeExpression(CompositeExpressionTypeAnd, parts...)
}

func NewOrCompositeExpression(parts ...*CompositeExpressionOrString) *CompositeExpression {
	return NewCompositeExpression(CompositeExpressionTypeOr, parts...)
}

// With - Returns a new CompositeExpression with the given parts added.
func (c *CompositeExpression) With(parts ...*CompositeExpressionOrString) *CompositeExpression {
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

	cloned.parts = make([]*CompositeExpressionOrString, len(c.parts))
	for i, part := range c.parts {
		cloned.parts[i] = part.Clone()
	}

	return &cloned
}
