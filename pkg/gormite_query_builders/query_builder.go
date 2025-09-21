package gormite_query_builders

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/KoNekoD/gormite/pkg/dtos"
	"github.com/KoNekoD/gormite/pkg/enums"
	"github.com/KoNekoD/gormite/pkg/expression_builders"
	"github.com/KoNekoD/gormite/pkg/g_err"
	gdh "github.com/KoNekoD/gormite/pkg/gormite_databases_helpers"
	"github.com/KoNekoD/gormite/pkg/platforms"
	"github.com/KoNekoD/gormite/pkg/platforms/postgres_platform"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	"slices"
	"strings"
)

// QueryBuilder - struct is responsible to dynamically create SQL queries.
//
// Important: Verify that every feature you use will work with your database vendor.
// SQL Query Builder does not attempt to validate the generated SQL at all.
//
// The query builder does no validation whatsoever if certain features even work with the
// underlying database vendor. Limit queries and joins are NOT applied to UPDATE and DELETE statements
// even if some vendors such as MySQL support it.
type QueryBuilder[ResultType any] struct {

	// sql - The complete SQL string for this query.
	sql *string

	// params - The query parameters.
	params map[string]any

	// types - The parameter type map of this query.
	types map[string]string

	// queryType - The type of query this is. Can be select, update or delete.
	queryType enums.QueryType

	// firstResult - The index of the first result to retrieve.
	firstResult int

	// maxResults - The maximum number of results to retrieve or NULL to retrieve all results.
	maxResults *int

	// boundCounter - The counter of bound parameters used with @see bindValue.
	boundCounter int

	// selectParts - The SELECT parts of the query.
	selectParts []string

	// returningParts - The RETURNING parts of the query.
	returningParts []string

	// distinct - Whether this is a SELECT DISTINCT query.
	distinct bool

	// fromParts - The FROM parts of a SELECT query.
	fromParts []*dtos.From

	// table - The table name for an INSERT, UPDATE or DELETE query.
	table *string

	// join - The list of joins, indexed by from alias.
	join map[string][]*dtos.Join

	// set - The SET parts of an UPDATE query.
	set []string

	// where - The WHERE part of a SELECT, UPDATE or DELETE query.
	where *dtos.CompositeExpressionOrString

	// groupBy - The GROUP BY part of a SELECT query.
	groupBy []string

	// having - The HAVING part of a SELECT query.
	having *dtos.CompositeExpressionOrString

	// orderBy - The ORDER BY parts of a SELECT query.
	orderBy []string

	// forUpdate - The FOR UPDATE part of a SELECT query.
	forUpdate *dtos.ForUpdate

	// values - The values of an INSERT query.
	values map[string]string

	// unionParts - The QueryBuilder for the union parts.
	unionParts []*dtos.Union

	Db         gdh.Database
	connection *platforms.Connection
	ctx        context.Context
}

func NewQueryBuilder[T any](db gdh.Database) *QueryBuilder[T] {
	return NewQueryBuilderWithContext[T](context.Background(), db)
}

func NewQueryBuilderWithContext[T any](
	ctx context.Context,
	db gdh.Database,
) *QueryBuilder[T] {
	return &QueryBuilder[T]{
		sql:          nil,
		params:       make(map[string]any),
		types:        make(map[string]string),
		queryType:    enums.QueryTypeSelect,
		firstResult:  0,
		maxResults:   nil,
		boundCounter: 0,
		selectParts:  nil,
		distinct:     false,
		fromParts:    nil,
		table:        nil,
		join:         make(map[string][]*dtos.Join),
		set:          make([]string, 0),
		where:        nil,
		groupBy:      nil,
		having:       nil,
		orderBy:      nil,
		forUpdate:    nil,
		values:       make(map[string]string),
		unionParts:   nil,
		connection: platforms.NewConnection(
			db,
			postgres_platform.NewPostgreSQLPlatform(),
		),
		Db:  db,
		ctx: ctx,
	}
}

// Expr - Gets an ExpressionBuilder used for object-oriented construction of query expressions.
// This producer method is intended for convenient inline usage
//
//		qb := NewQueryBuilder(db).Select("u.id").From("users", "u")
//	 qb.Where(qb.Expr().Eq("u.id", "1"));
//
// For more complex expression construction, consider storing the expression
// builder object in a local variable.
func (qb *QueryBuilder[ResultType]) Expr() *expression_builders.ExpressionBuilder {
	return &expression_builders.ExpressionBuilder{}
}

// GetSQL - Gets the complete SQL string formed by the current specifications of this QueryBuilder.
// <code>
//
//	qb = em.createQueryBuilder()
//	    .select('u')
//	    .from('User', 'u')
//	echo qb.getSQL(); // SELECT u FROM User u
//
// </code>
// @return string The SQL query string.
func (qb *QueryBuilder[ResultType]) GetSQL() (string, error) {
	var err error
	if qb.sql == nil {
		var resultSql string
		switch qb.queryType {
		case enums.QueryTypeInsert:
			resultSql = qb.getSQLForInsert()
		case enums.QueryTypeDelete:
			resultSql = qb.GetSQLForDelete()
		case enums.QueryTypeUpdate:
			resultSql = qb.GetSQLForUpdate()
		case enums.QueryTypeSelect:
			resultSql, err = qb.GetSQLForSelect()
		case enums.QueryTypeUnion:
			resultSql, err = qb.GetSQLForUnion()
		default:
			return "", errors.New("invalid query type")
		}
		qb.sql = &resultSql
	}
	return *qb.sql, err
}

func (qb *QueryBuilder[ResultType]) MustGetSQL() string {
	gotSql, err := qb.GetSQL()
	if err != nil {
		panic(err)
	}

	return gotSql
}

// SetParameter - Sets a query parameter for the query being constructed.
//
//	qb = conn.createQueryBuilder().select('u').from('users', 'u').where('u.id = :user_id').setParameter('user_id', 1);
func (qb *QueryBuilder[ResultType]) SetParameter(
	key string,
	value any,
	paramType ...enums.ParameterType,
) *QueryBuilder[ResultType] {
	qb.params[key] = value
	paramTypeItem := enums.ParameterTypeString
	if len(paramType) > 0 {
		if len(paramType) > 1 {
			panic(errors.New("Only one parameter type is allowed"))
		}
		paramTypeItem = paramType[0]
	}
	qb.types[key] = string(paramTypeItem)
	return qb
}

// SetParameters - Sets a collection of query parameters for the query being constructed.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u')
//	    .from('users', 'u')
//	    .where('u.id = :user_id1 OR u.id = :user_id2')
//	    .setParameters(array(
//	        'user_id1' => 1,
//	        'user_id2' => 2
//	    ));
//
// </code>
func (qb *QueryBuilder[ResultType]) SetParameters(
	params map[string]any,
	types ...map[string]string,
) *QueryBuilder[ResultType] {
	qb.params = params

	if len(types) > 0 {
		if len(types) > 1 {
			panic(errors.New("Only one parameter type is allowed"))
		}
		typesItem := types[0]
		qb.types = typesItem
	} else {
		qb.types = make(map[string]string)
		for key := range params {
			qb.types[key] = string(enums.ParameterTypeString)
		}
	}

	return qb
}

// GetParameter - Gets a (previously set) query parameter of the query being constructed.
// @param string|int key The key (index or name) of the bound parameter.
// @return mixed The value of the bound parameter.
func (qb *QueryBuilder[ResultType]) GetParameter(key string) any {
	if value, ok := qb.params[key]; ok {
		return value
	}
	return nil
}

// GetNamedArgs - Gets all defined query parameters of the query being constructed indexed by parameter name.
func (qb *QueryBuilder[ResultType]) GetNamedArgs() any {
	args := map[string]any{}
	for _, key := range maps.Keys(qb.GetParameterTypes()) {
		param := qb.GetParameter(key)
		args[key] = param
	}
	return qb.Db.GetNamedArgs(args)
}

// GetParameterTypes - Gets all defined query parameter types for the query being constructed indexed by parameter index or name.
func (qb *QueryBuilder[ResultType]) GetParameterTypes() map[string]string {
	return qb.types
}

// GetParameterType - Gets a (previously set) query parameter type of the query being constructed.
// @param int|string key The key of the bound parameter type
func (qb *QueryBuilder[ResultType]) GetParameterType(key string) string {
	if paramType, ok := qb.types[key]; ok {
		return paramType
	}
	return "STRING" // Default type if not found
}

// SetFirstResult - Sets the position of the first result to retrieve (the "offset").
func (qb *QueryBuilder[ResultType]) SetFirstResult(firstResult int) *QueryBuilder[ResultType] {
	qb.firstResult = firstResult
	qb.sql = nil
	return qb
}

// GetFirstResult - Gets the position of the first result the query object was set to retrieve (the "offset").
func (qb *QueryBuilder[ResultType]) GetFirstResult() int {
	return qb.firstResult
}

// SetMaxResults - Sets the maximum number of results to retrieve (the "limit").
func (qb *QueryBuilder[ResultType]) SetMaxResults(maxResults int) *QueryBuilder[ResultType] {
	qb.maxResults = &maxResults
	qb.sql = nil
	return qb
}

// GetMaxResults - Gets the maximum number of results the query object was set to retrieve (the "limit").
// Returns NULL if all results will be returned.
func (qb *QueryBuilder[ResultType]) GetMaxResults() *int {
	return qb.maxResults
}

// ForUpdate - Locks the queried rows for a subsequent update.
// Default: ConflictResolutionModeOrdinary
func (qb *QueryBuilder[ResultType]) ForUpdate(conflictResolutionMode enums.ConflictResolutionMode) *QueryBuilder[ResultType] {
	qb.forUpdate = dtos.NewForUpdate(conflictResolutionMode)
	qb.sql = nil
	return qb
}

// Union - Specifies union parts to be used to build a UNION query.
// Replaces any previously specified parts.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .union('SELECT 1 AS field1', 'SELECT 2 AS field1');
//
// </code>
func (qb *QueryBuilder[ResultType]) Union(part *dtos.QueryBuilderOrString) *QueryBuilder[ResultType] {
	qb.queryType = enums.QueryTypeUnion
	qb.unionParts = []*dtos.Union{dtos.NewUnion(part)}
	qb.sql = nil
	return qb
}

// AddUnion - Add parts to be used to build a UNION query.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .union('SELECT 1 AS field1')
//	    .addUnion('SELECT 2 AS field1', 'SELECT 3 AS field1')
//
// </code>
// unionType - by default UnionTypeDistinct
func (qb *QueryBuilder[ResultType]) AddUnion(
	part *dtos.QueryBuilderOrString,
	unionType enums.UnionType,
) *QueryBuilder[ResultType] {
	qb.queryType = enums.QueryTypeUnion
	if len(qb.unionParts) == 0 {
		panic(errors.New("No initial UNION part set, use Union() to set one first"))
	}
	qb.unionParts = append(
		qb.unionParts,
		dtos.NewUnionWithType(part, unionType),
	)
	qb.sql = nil
	return qb
}

// Select - Specifies an item that is to be returned in the query result.
// Replaces any previously specified selections, if any.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.id', 'p.id')
//	    .from('users', 'u')
//	    .leftJoin('u', 'phonenumbers', 'p', 'u.id = p.user_id');
//
// </code>
// @param string ...expressions The selection expressions.
func (qb *QueryBuilder[ResultType]) Select(expressions ...string) *QueryBuilder[ResultType] {
	qb.queryType = enums.QueryTypeSelect
	qb.selectParts = expressions
	qb.sql = nil
	return qb
}

func (qb *QueryBuilder[ResultType]) Returning(expressions ...string) *QueryBuilder[ResultType] {
	qb.returningParts = expressions
	qb.sql = nil
	return qb
}

func (qb *QueryBuilder[ResultType]) AddReturning(expressions ...string) *QueryBuilder[ResultType] {
	qb.returningParts = append(qb.returningParts, expressions...)
	qb.sql = nil
	return qb
}

// Distinct - Adds or removes DISTINCT to/from the query.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.id')
//	    .distinct()
//	    .from('users', 'u')
//
// </code>
func (qb *QueryBuilder[ResultType]) Distinct(distinct bool) *QueryBuilder[ResultType] {
	qb.distinct = distinct
	qb.sql = nil
	return qb
}

// AddSelect - Adds an item that is to be returned in the query result.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.id')
//	    .addSelect('p.id')
//	    .from('users', 'u')
//	    .leftJoin('u', 'phonenumbers', 'u.id = p.user_id');
//
// </code>
// @param string expression     The selection expressions.
func (qb *QueryBuilder[ResultType]) AddSelect(expressions ...string) *QueryBuilder[ResultType] {
	qb.queryType = enums.QueryTypeSelect
	qb.selectParts = append(qb.selectParts, expressions...)
	qb.sql = nil
	return qb
}

// Delete - Turns the query being built into a bulk delete query that ranges over
// a certain table.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .delete('users u')
//	    .where('u.id = :user_id')
//	    .setParameter(':user_id', 1);
//
// </code>
// @param string table The table whose rows are subject to the deletion.
func (qb *QueryBuilder[ResultType]) Delete(table string) *QueryBuilder[ResultType] {
	qb.queryType = enums.QueryTypeDelete
	qb.table = &table
	qb.sql = nil
	return qb
}

// Update - Turns the query being built into a bulk update query that ranges over
// a certain table
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .update('counters c')
//	    .set('c.value', 'c.value + 1')
//	    .where('c.id = ?');
//
// </code>
// @param string table The table whose rows are subject to the update.
func (qb *QueryBuilder[ResultType]) Update(table string) *QueryBuilder[ResultType] {
	qb.queryType = enums.QueryTypeUpdate
	qb.table = &table
	qb.sql = nil
	return qb
}

// Insert - Turns the query being built into an insert query that inserts into
// a certain table
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .insert('users')
//	    .values(
//	        array(
//	            'name' => '?',
//	            'password' => '?'
//	        )
//	    );
//
// </code>
// @param string table The table into which the rows should be inserted.
func (qb *QueryBuilder[ResultType]) Insert(table string) *QueryBuilder[ResultType] {
	qb.queryType = enums.QueryTypeInsert
	qb.table = &table
	qb.sql = nil
	return qb
}

// From - Creates and adds a query root corresponding to the table identified by the
// given alias, forming a cartesian product with any existing query roots.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.id')
//	    .from('users', 'u')
//
// </code>
// @param string      table The table.
// @param string|null alias The alias of the table.
func (qb *QueryBuilder[ResultType]) From(
	table string,
	alias ...string,
) *QueryBuilder[ResultType] {
	var aliasItem *string
	if len(alias) > 0 {
		if len(alias) > 1 {
			panic(errors.New("Only one alias is allowed"))
		}
		aliasItem = &alias[0]
	}

	qb.fromParts = append(qb.fromParts, dtos.NewFrom(table, aliasItem))
	qb.sql = nil
	return qb
}

// Join - Creates and adds a join to the query.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.name')
//	    .from('users', 'u')
//	    .join('u', 'phonenumbers', 'p', 'p.is_primary = 1');
//
// </code>
// @param string fromAlias The alias that points to a from clause.
// @param string join      The table name to join.
// @param string alias     The alias of the join table.
// @param string condition The condition for the join.
func (qb *QueryBuilder[ResultType]) Join(fromAlias, join, alias, condition string) *QueryBuilder[ResultType] {
	return qb.InnerJoin(fromAlias, join, alias, condition)
}

func (qb *QueryBuilder[ResultType]) initJoinIfNeeded(fromAlias string) {
	if qb.join == nil {
		qb.join = make(map[string][]*dtos.Join)
	}

	if _, ok := qb.join[fromAlias]; !ok {
		qb.join[fromAlias] = make([]*dtos.Join, 0)
	}
}

// InnerJoin - Creates and adds a join to the query.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.name')
//	    .from('users', 'u')
//	    .innerJoin('u', 'phonenumbers', 'p', 'p.is_primary = 1');
//
// </code>
// @param string fromAlias The alias that points to a from clause.
// @param string join      The table name to join.
// @param string alias     The alias of the join table.
// @param string condition The condition for the join.
func (qb *QueryBuilder[ResultType]) InnerJoin(fromAlias, join, alias, condition string) *QueryBuilder[ResultType] {
	qb.initJoinIfNeeded(fromAlias)
	qb.join[fromAlias] = append(
		qb.join[fromAlias],
		dtos.NewInnerJoin(join, alias, &condition),
	)
	qb.sql = nil
	return qb
}

// LeftJoin - Creates and adds a left join to the query.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.name')
//	    .from('users', 'u')
//	    .leftJoin('u', 'phonenumbers', 'p', 'p.is_primary = 1');
//
// </code>
// @param string fromAlias The alias that points to a from clause.
// @param string join      The table name to join.
// @param string alias     The alias of the join table.
// @param string condition The condition for the join.
func (qb *QueryBuilder[ResultType]) LeftJoin(fromAlias, join, alias, condition string) *QueryBuilder[ResultType] {
	qb.join[fromAlias] = append(
		qb.join[fromAlias],
		dtos.NewLeftJoin(join, alias, &condition),
	)
	qb.sql = nil
	return qb
}

// RightJoin - Creates and adds a right join to the query.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.name')
//	    .from('users', 'u')
//	    .rightJoin('u', 'phonenumbers', 'p', 'p.is_primary = 1');
//
// </code>
// @param string fromAlias The alias that points to a from clause.
// @param string join      The table name to join.
// @param string alias     The alias of the join table.
// @param string condition The condition for the join.
func (qb *QueryBuilder[ResultType]) RightJoin(fromAlias, join, alias, condition string) *QueryBuilder[ResultType] {
	qb.join[fromAlias] = append(
		qb.join[fromAlias],
		dtos.NewRightJoin(join, alias, &condition),
	)
	qb.sql = nil
	return qb
}

// Set - Sets a new value for a column in a bulk update query.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .update('counters c')
//	    .set('c.value', 'c.value + 1')
//	    .where('c.id = ?');
//
// </code>
// @param string key   The column to set.
// @param string value The value, expression, placeholder, etc.
func (qb *QueryBuilder[ResultType]) Set(key, value string) *QueryBuilder[ResultType] {
	qb.set = append(qb.set, key+" = "+value)
	qb.sql = nil
	return qb
}

// Where - Specifies one or more restrictions to the query result.
// Replaces any previously specified restrictions, if any.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('c.value')
//	    .from('counters', 'c')
//	    .where('c.id = ?');
//	// You can optionally programmatically build and/or expressions
//	qb = conn.createQueryBuilder();
//	or = qb.expr().orx();
//	or.add(qb.expr().eq('c.id', 1));
//	or.add(qb.expr().eq('c.id', 2));
//	qb.update('counters c')
//	    .set('c.value', 'c.value + 1')
//	    .where(or);
//
// </code>
// @param string|CompositeExpression predicate     The WHERE clause predicates.
func (qb *QueryBuilder[ResultType]) Where(predicates ...string) *QueryBuilder[ResultType] {
	predicatesItem := make([]*dtos.CompositeExpressionOrString, 0)
	for _, s := range predicates {
		predicatesItem = append(
			predicatesItem,
			&dtos.CompositeExpressionOrString{String: &s},
		)
	}

	qb.where = qb.createPredicate(predicatesItem...)
	qb.sql = nil
	return qb
}

func (qb *QueryBuilder[ResultType]) WhereViaExpr(predicates ...*dtos.CompositeExpressionOrString) *QueryBuilder[ResultType] {
	qb.where = qb.createPredicate(predicates...)
	qb.sql = nil
	return qb
}

// AndWhere - Adds one or more restrictions to the query results, forming a logical
// conjunction with any previously specified restrictions.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u')
//	    .from('users', 'u')
//	    .where('u.username LIKE ?')
//	    .andWhere('u.is_active = 1');
//
// </code>
// @see where()
// @param string|CompositeExpression predicate     The predicates to append.
func (qb *QueryBuilder[ResultType]) AndWhere(predicates ...string) *QueryBuilder[ResultType] {
	predicatesItem := make([]*dtos.CompositeExpressionOrString, 0)
	for _, s := range predicates {
		predicatesItem = append(
			predicatesItem,
			&dtos.CompositeExpressionOrString{String: &s},
		)
	}

	qb.where = qb.appendToPredicate(
		qb.where,
		dtos.CompositeExpressionTypeAnd,
		predicatesItem...,
	)
	qb.sql = nil
	return qb
}

func (qb *QueryBuilder[ResultType]) AndWhereViaExpr(predicates ...*dtos.CompositeExpressionOrString) *QueryBuilder[ResultType] {
	qb.where = qb.appendToPredicate(
		qb.where,
		dtos.CompositeExpressionTypeAnd,
		predicates...,
	)
	qb.sql = nil
	return qb
}

// OrWhere - Adds one or more restrictions to the query results, forming a logical
// disjunction with any previously specified restrictions.
// <code>
//
//	qb = em.createQueryBuilder()
//	    .select('u.name')
//	    .from('users', 'u')
//	    .where('u.id = 1')
//	    .orWhere('u.id = 2');
//
// </code>
// @see where()
// @param string|CompositeExpression predicate     The predicates to append.
func (qb *QueryBuilder[ResultType]) OrWhere(predicates ...*dtos.CompositeExpressionOrString) *QueryBuilder[ResultType] {
	qb.where = qb.appendToPredicate(
		qb.where,
		dtos.CompositeExpressionTypeOr,
		predicates...,
	)
	qb.sql = nil
	return qb
}

// GroupBy - Specifies one or more grouping expressions over the results of the query.
// Replaces any previously specified groupings, if any.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.name')
//	    .from('users', 'u')
//	    .groupBy('u.id');
//
// </code>
// @param string expression     The grouping expressions
func (qb *QueryBuilder[ResultType]) GroupBy(expressions ...string) *QueryBuilder[ResultType] {
	qb.groupBy = append([]string{}, expressions...)
	qb.sql = nil
	return qb
}

// AddGroupBy - Adds one or more grouping expressions to the query.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .select('u.name')
//	    .from('users', 'u')
//	    .groupBy('u.lastLogin')
//	    .addGroupBy('u.createdAt');
//
// </code>
// @param string expression     The grouping expressions
func (qb *QueryBuilder[ResultType]) AddGroupBy(expressions ...string) *QueryBuilder[ResultType] {
	qb.groupBy = append(qb.groupBy, expressions...)
	qb.sql = nil
	return qb
}

// SetValue - Sets a value for a column in an insert query.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .insert('users')
//	    .values(
//	        array(
//	            'name' => '?'
//	        )
//	    )
//	    .setValue('password', '?');
//
// </code>
// @param string column The column into which the value should be inserted.
// @param string value  The value that should be inserted into the column.
func (qb *QueryBuilder[ResultType]) SetValue(column, value string) *QueryBuilder[ResultType] {
	qb.values[column] = value
	return qb
}

// Values - Specifies values for an insert query indexed by column names.
// Replaces any previous values, if any.
// <code>
//
//	qb = conn.createQueryBuilder()
//	    .insert('users')
//	    .values(
//	        array(
//	            'name' => '?',
//	            'password' => '?'
//	        )
//	    );
//
// </code>
// @param array<string, mixed> values The values to specify for the insert query indexed by column names.
func (qb *QueryBuilder[ResultType]) Values(values map[string]string) *QueryBuilder[ResultType] {
	qb.values = values
	qb.sql = nil
	return qb
}

// Having - Specifies a restriction over the groups of the query.
// Replaces any previous having restrictions, if any.
// @param string|CompositeExpression predicate     The HAVING clause predicates.
func (qb *QueryBuilder[ResultType]) Having(predicates ...*dtos.CompositeExpressionOrString) *QueryBuilder[ResultType] {
	qb.having = qb.createPredicate(predicates...)
	qb.sql = nil
	return qb
}

// AndHaving - Adds a restriction over the groups of the query, forming a logical
// conjunction with any existing having restrictions.
// @param string|CompositeExpression predicate     The predicates to append.
func (qb *QueryBuilder[ResultType]) AndHaving(predicates ...*dtos.CompositeExpressionOrString) *QueryBuilder[ResultType] {
	qb.having = qb.appendToPredicate(
		qb.having,
		dtos.CompositeExpressionTypeAnd,
		predicates...,
	)
	qb.sql = nil
	return qb
}

// OrHaving - Adds a restriction over the groups of the query, forming a logical
// disjunction with any existing having restrictions.
// @param string|CompositeExpression predicate     The predicates to append.
func (qb *QueryBuilder[ResultType]) OrHaving(predicates ...*dtos.CompositeExpressionOrString) *QueryBuilder[ResultType] {
	qb.having = qb.appendToPredicate(
		qb.having,
		dtos.CompositeExpressionTypeOr,
		predicates...,
	)
	qb.sql = nil
	return qb
}

// createPredicate - Creates a CompositeExpression from one or more predicates combined by the AND logic.
func (qb *QueryBuilder[ResultType]) createPredicate(predicates ...*dtos.CompositeExpressionOrString) *dtos.CompositeExpressionOrString {
	if len(predicates) == 1 {
		return predicates[0]
	}

	return &dtos.CompositeExpressionOrString{CompositeExpression: dtos.NewAndCompositeExpression(predicates...)}
}

// appendToPredicate - Appends the given predicates combined by the given type of logic to the current predicate.
func (qb *QueryBuilder[ResultType]) appendToPredicate(
	currentPredicate *dtos.CompositeExpressionOrString,
	exprType dtos.CompositeExpressionType,
	predicates ...*dtos.CompositeExpressionOrString,
) *dtos.CompositeExpressionOrString {
	if currentPredicate != nil && currentPredicate.CompositeExpression != nil && currentPredicate.CompositeExpression.GetType() == string(exprType) {
		return &dtos.CompositeExpressionOrString{CompositeExpression: currentPredicate.CompositeExpression.With(predicates...)}
	}

	if currentPredicate != nil {
		predicates = append(
			[]*dtos.CompositeExpressionOrString{currentPredicate},
			predicates...,
		)
	} else if len(predicates) == 1 {
		return predicates[0]
	}

	return &dtos.CompositeExpressionOrString{
		CompositeExpression: dtos.NewCompositeExpression(
			exprType,
			predicates...,
		),
	}
}

// OrderBy - Specifies an ordering for the query results.
// Replaces any previously specified orderings, if any.
// @param string sort  The ordering expression.
// @param string order The ordering direction.
func (qb *QueryBuilder[ResultType]) OrderBy(
	sort string,
	order ...string,
) *QueryBuilder[ResultType] {
	orderBy := sort
	if len(order) > 0 {
		if len(order) > 1 {
			panic(errors.New("Only one order is allowed"))
		}
		orderBy += " " + order[0]
	}
	qb.orderBy = []string{orderBy}
	qb.sql = nil
	return qb
}

// AddOrderBy - Adds an ordering to the query results.
// @param string sort  The ordering expression.
// @param string order The ordering direction.
func (qb *QueryBuilder[ResultType]) AddOrderBy(
	sort string,
	order *string,
) *QueryBuilder[ResultType] {
	orderBy := sort
	if order != nil {
		orderBy += " " + *order
	}
	qb.orderBy = append(qb.orderBy, orderBy)
	qb.sql = nil
	return qb
}

// ResetWhere - Resets the WHERE conditions for the query.
func (qb *QueryBuilder[ResultType]) ResetWhere() *QueryBuilder[ResultType] {
	qb.where = nil
	qb.sql = nil
	return qb
}

// ResetGroupBy - Resets the grouping for the query.
func (qb *QueryBuilder[ResultType]) ResetGroupBy() *QueryBuilder[ResultType] {
	qb.groupBy = make([]string, 0)
	qb.sql = nil
	return qb
}

// ResetHaving - Resets the HAVING conditions for the query.
func (qb *QueryBuilder[ResultType]) ResetHaving() *QueryBuilder[ResultType] {
	qb.having = nil
	qb.sql = nil
	return qb
}

// ResetOrderBy - Resets the ordering for the query.
func (qb *QueryBuilder[ResultType]) ResetOrderBy() *QueryBuilder[ResultType] {
	qb.orderBy = make([]string, 0)
	qb.sql = nil
	return qb
}

// GetSQLForSelect generates SQL for a SELECT query.
func (qb *QueryBuilder[ResultType]) GetSQLForSelect() (string, error) {
	if len(qb.selectParts) == 0 {
		return "", g_err.NewQueryException("No SELECT expressions given. Please use select() or addSelect().")
	}

	fromClauses, err := qb.getFromClauses()
	if err != nil {
		return "", err
	}

	var where *string
	var having *string

	if qb.where != nil {
		whereTmp := qb.where.ToString()
		where = &whereTmp
	}
	if qb.having != nil {
		havingTmp := qb.having.ToString()
		having = &havingTmp
	}

	platform := qb.connection.GetDatabasePlatform()

	return platform.CreateSelectSQLBuilder().
		BuildSQL(
			dtos.NewSelectQuery(
				qb.distinct,
				qb.selectParts,
				maps.Values(fromClauses),
				where,
				qb.groupBy,
				having,
				qb.orderBy,
				dtos.NewLimit(qb.maxResults, qb.firstResult),
				qb.forUpdate,
			),
		)
}

func (qb *QueryBuilder[ResultType]) getFromClauses() (
	map[string]string,
	error,
) {
	fromClauses := make(map[string]string)
	knownAliases := make(map[string]bool)
	for _, from := range qb.fromParts {
		var (
			tableSql       string
			tableReference string
		)

		if from.GetAlias() == nil || *from.GetAlias() == from.GetTable() {
			tableSql = from.GetTable()
			tableReference = from.GetTable()
		} else {
			tableSql = from.GetTable() + " " + *from.GetAlias()
			tableReference = *from.GetAlias()
		}

		knownAliases[tableReference] = true

		sqlForJoins, err := qb.GetSQLForJoins(tableReference, knownAliases)
		if err != nil {
			return nil, err
		}
		fromClauses[tableReference] = tableSql + sqlForJoins

	}

	err := qb.verifyAllAliasesAreKnown(knownAliases)
	if err != nil {
		return nil, err
	}

	return fromClauses, nil
}

func (qb *QueryBuilder[ResultType]) verifyAllAliasesAreKnown(knownAliases map[string]bool) error {
	for fromAlias := range qb.join {
		if !knownAliases[fromAlias] {
			return g_err.NewUnknownAlias(
				fromAlias,
				maps.Keys(knownAliases),
			)
		}
	}
	return nil
}

// GetSQLForUnion generates a SQL string for a UNION query.
func (qb *QueryBuilder[ResultType]) GetSQLForUnion() (string, error) {
	if len(qb.unionParts) < 2 {
		return "", errors.New("insufficient UNION parts, need at least 2")
	}
	platform := qb.connection.GetDatabasePlatform()
	return platform.CreateUnionSQLBuilder().BuildSQL(
		dtos.NewUnionQuery(
			qb.unionParts,
			qb.orderBy,
			dtos.NewLimit(qb.maxResults, qb.firstResult),
		),
	)
}

// getSQLForInsert - Converts this instance into an INSERT string in SQL.
func (qb *QueryBuilder[ResultType]) getSQLForInsert() string {
	returningSql := ""
	if qb.returningParts != nil && len(qb.returningParts) > 0 {
		returningSql = " RETURNING " + strings.Join(qb.returningParts, ", ")
	}

	keys := maps.Keys(qb.values)
	slices.Sort(keys)

	values := make([]string, 0, len(qb.values))
	for _, key := range keys {
		values = append(values, qb.values[key])
	}

	return fmt.Sprintf(
		"INSERT INTO"+" %s (%s) VALUES (%s)%s",
		*qb.table,
		strings.Join(keys, ", "),
		strings.Join(values, ", "),
		returningSql,
	)
}

// GetSQLForUpdate - Converts this instance into an UPDATE string in SQL.
func (qb *QueryBuilder[ResultType]) GetSQLForUpdate() string {
	returningSql := ""
	if qb.returningParts != nil && len(qb.returningParts) > 0 {
		returningSql = " RETURNING " + strings.Join(qb.returningParts, ", ")
	}
	query := fmt.Sprintf(
		"UPDATE"+" "+"%s SET %s",
		*qb.table,
		strings.Join(qb.set, ", "),
	)
	if qb.where != nil {
		query += " WHERE " + qb.where.ToString()
	}
	if returningSql != "" {
		query += returningSql
	}
	return query
}

// GetSQLForDelete - Converts this instance into a DELETE string in SQL.
func (qb *QueryBuilder[ResultType]) GetSQLForDelete() string {
	query := fmt.Sprintf("DELETE"+" "+"FROM %s", *qb.table)
	if qb.where != nil {
		query += " WHERE " + qb.where.ToString()
	}
	return query
}

// ToString - Gets a string representation of this QueryBuilder which corresponds to
// the final SQL query being constructed.
// @return string The string representation of this QueryBuilder.
func (qb *QueryBuilder[ResultType]) ToString() string {
	gotSQL, _ := qb.GetSQL()
	return gotSQL
}

// CreateNamedParameter creates a new named parameter and bind the value to it.
// If placeholder is not provided createNamedParameter() will automatically
// create a placeholder for you. An automatic placeholder will be of the
// name ':dcValue1', ':dcValue2' etc.
func (qb *QueryBuilder[ResultType]) CreateNamedParameter(
	value any,
	paramType enums.ParameterType,
	placeHolder *string,
) string {
	if placeHolder == nil {
		qb.boundCounter++
		ph := fmt.Sprintf(":dcValue%d", qb.boundCounter)
		placeHolder = &ph
	}
	qb.SetParameter(strings.TrimPrefix(*placeHolder, ":"), value, paramType)
	return *placeHolder
}

// CreatePositionalParameter creates a new positional parameter and bind the given value to it
func (qb *QueryBuilder[ResultType]) CreatePositionalParameter(value any, paramType ...enums.ParameterType) string {
	param := fmt.Sprintf("%d", qb.boundCounter)
	qb.SetParameter(param, value, paramType...)
	qb.boundCounter++
	return param
}

// GetSQLForJoins generates SQL for the JOIN clauses
func (qb *QueryBuilder[ResultType]) GetSQLForJoins(fromAlias string, knownAliases map[string]bool) (string, error) {
	var sqlBuilder strings.Builder
	if qb.join[fromAlias] == nil {
		return "", nil
	}
	for _, join := range qb.join[fromAlias] {
		if knownAliases[join.GetAlias()] {
			return "", g_err.NewNonUniqueAlias(join.GetAlias(), maps.Keys(knownAliases))
		}
		sqlBuilder.WriteString(fmt.Sprintf(" %s JOIN %s %s", join.GetType(), join.GetTable(), join.GetAlias()))
		if join.GetCondition() != nil {
			sqlBuilder.WriteString(fmt.Sprintf(" ON %s", *join.GetCondition()))
		}

		knownAliases[join.GetAlias()] = true
	}

	for _, join := range qb.join[fromAlias] {
		joinSQL, err := qb.GetSQLForJoins(join.GetAlias(), knownAliases)
		if err != nil {
			return "", err
		}
		sqlBuilder.WriteString(joinSQL)
	}

	return sqlBuilder.String(), nil
}

// Clone deep clone of all expression objects in the SQL parts
func (qb *QueryBuilder[ResultType]) Clone() *QueryBuilder[ResultType] {
	cloned := *qb
	cloned.fromParts = make([]*dtos.From, len(qb.fromParts))
	for i, from := range qb.fromParts {
		cloned.fromParts[i] = dtos.NewFrom(from.GetTable(), from.GetAlias())
	}
	cloned.join = make(map[string][]*dtos.Join)
	for alias, joins := range qb.join {
		cloned.join[alias] = make([]*dtos.Join, len(joins))
		for i, join := range joins {
			cloned.join[alias][i] = dtos.NewJoin(join.GetType(), join.GetTable(), join.GetAlias(), join.GetCondition())
		}
	}
	if qb.where != nil {
		cloned.where = qb.where.Clone()
	}
	if qb.having != nil {
		cloned.having = qb.having.Clone()
	}
	cloned.params = make(map[string]any)
	for key, param := range qb.params {
		cloned.params[key] = param
	}
	return &cloned
}

// PrepareIN creates a string of named parameters for the IN clause of the query
func (qb *QueryBuilder[ResultType]) PrepareIN(args []string) string {
	namedArgs := make([]string, 0, len(args))
	for i := range args {
		namedArgs = append(
			namedArgs,
			qb.CreateNamedParameter(args[i], enums.ParameterTypeString, nil),
		)
	}

	return strings.Join(namedArgs, ", ")
}

// PrepareInArgsInt creates int named parameters for the IN clause of the query
func (qb *QueryBuilder[ResultType]) PrepareInArgsInt(args []int) []string {
	namedArgs := make([]string, 0, len(args))
	for i := range args {
		namedArgs = append(
			namedArgs,
			qb.CreateNamedParameter(args[i], enums.ParameterTypeString, nil),
		)
	}
	return namedArgs
}

// GetRootAliases returns the root aliases of the query
func (qb *QueryBuilder[ResultType]) GetRootAliases() []string {
	aliases := make([]string, 0)

	for _, part := range qb.fromParts {
		itemAlias := part.GetTable()

		if alias := part.GetAlias(); alias != nil {
			itemAlias = *alias
		}

		aliases = append(aliases, itemAlias)
	}

	return aliases
}

// Exec executes the query
func (qb *QueryBuilder[ResultType]) Exec() error {
	gotSql, err := qb.GetSQL()
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = qb.Db.Exec(qb.ctx, gotSql, qb.GetNamedArgs())
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// ExecScan scans the first row of the result set into the target variable
func (qb *QueryBuilder[ResultType]) ExecScan(v any) error {
	gotSql, err := qb.GetSQL()
	if err != nil {
		return errors.WithStack(err)
	}

	err = qb.Db.Select(gotSql, qb.GetNamedArgs()).Scan(v).Exec(qb.ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// ExecScanCol scans the column of the first row of the result set into the target variable
func (qb *QueryBuilder[ResultType]) ExecScanCol(target any) error {
	gotSql, err := qb.GetSQL()
	if err != nil {
		return errors.WithStack(err)
	}

	err = qb.Db.Select(gotSql, qb.GetNamedArgs()).ScanCol(target).Exec(qb.ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// GetResult returns a result in the form of a ResultType slice
func (qb *QueryBuilder[ResultType]) GetResult() ([]*ResultType, error) {
	v := make([]*ResultType, 0)

	if err := qb.ExecScan(&v); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, errors.WithStack(err)
	}

	return v, nil
}

// GetOneOrNilResult reruns a result in the form of a ResultType
func (qb *QueryBuilder[ResultType]) GetOneOrNilResult() (*ResultType, error) {
	var v []ResultType

	err := qb.ExecScan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(v) == 0 {
		return nil, nil
	}

	if len(v) > 1 {
		return nil, errors.Errorf("expected 1 result, got %d", len(v))
	}

	return &v[0], nil
}

// GetOneOrNilLiteralResult returns a literal result in the form of a ResultType
func (qb *QueryBuilder[ResultType]) GetOneOrNilLiteralResult() (*ResultType, error) {
	gotSql, err := qb.GetSQL()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	args := qb.GetNamedArgs()

	var v ResultType

	err = qb.Db.Select(gotSql, args).ScanCol(&v).Exec(qb.ctx)
	if err != nil {
		return &v, errors.WithStack(err)
	}

	return &v, nil
}

// GetLiteralResult returns a literal set of results in the form of a ResultType slice
func (qb *QueryBuilder[ResultType]) GetLiteralResult() ([]ResultType, error) {
	gotSql, err := qb.GetSQL()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var v []ResultType

	rows, err := qb.Db.Query(qb.ctx, gotSql, qb.GetNamedArgs())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for rows.Next() {
		var rowValue ResultType

		if err := rows.Scan(&rowValue); err != nil {
			return nil, errors.WithStack(err)
		}

		v = append(v, rowValue)
	}

	return v, nil
}
