package dao

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ansel1/merry"
	utils "github.com/coachbit/gorm-dao/dao/daoutils"
	"github.com/jinzhu/gorm"
	"github.com/tkrajina/go-reflector/reflector"
)

const defaultPageSize = 50

type QueryIterator struct {
	db   *gorm.DB
	rows *sql.Rows
}

func (qi QueryIterator) Next() bool {
	next := qi.rows.Next()
	if !next {
		qi.Close()
	}
	return next
}

func (qi QueryIterator) Scan(target Model) error {
	if err := qi.db.ScanRows(qi.rows, target); err != nil {
		return merry.Wrap(err)
	}
	return nil
}

// Close closes the rows. It will be called automatically if iterator is iterated until the last line.
func (qi QueryIterator) Close() {
	if err := qi.rows.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error closing rows: %#v", err)
	}
}

type exprValue struct {
	val interface{}
}

func ExprValue(val interface{}) interface{} {
	return exprValue{val}
}

type Query struct {
	gormDb         *gorm.DB
	logger         Logger
	c              context.Context
	statsCollector DbStatsCollector
	err            error

	pageNo         int
	pageSize       int
	includeDeleted bool
	logStr         []string

	expressions []string
	values      []interface{}

	orderBy []string
}

func (q *Query) IncludeDeleted() *Query {
	q.includeDeleted = true
	return q
}

func (q *Query) RawRows(sql string, values ...interface{}) (*sql.Rows, error) {
	rows, err := q.gormDb.Raw(sql, values...).Rows()
	if err != nil {
		return nil, merry.Wrap(err).Appendf("sql: %v, values= %#v", sql, values)
	}
	return rows, nil
}

func (q *Query) appendFilterExpressionAndValues(descr, expression string, values ...interface{}) {
	if strings.Count(expression, "?") != len(values) {
		q.err = merry.New("invalid expression placeholders count").Appendf("'%s' params: %#v", expression, values)
	}
	q.logStr = append(q.logStr, descr+expression)
	if expression != "" {
		q.expressions = append(q.expressions, expression)
	}
	q.values = append(q.values, values...)
}

func (q *Query) FilterRawExpression(expression string, values ...interface{}) *Query {
	q.appendFilterExpressionAndValues("filter", expression, values...)
	return q
}

func (q *Query) Filter(column, operation string, value interface{}) *Query {
	q.appendFilterExpressionAndValues("filter", column+operation+"?", value)
	return q
}

func (q *Query) FilterInStrings(column string, valuesStrs ...string) *Query {
	values := make([]interface{}, len(valuesStrs))
	for n := range valuesStrs {
		values[n] = valuesStrs[n]
	}
	return q.FilterIn(column, values...)
}

func (q *Query) FilterIsNotNull(column string) *Query {
	q.appendFilterExpressionAndValues("not_null", column+" is not null")
	return q
}

func (q *Query) FilterIn(column string, values ...interface{}) *Query {
	valuesMap := map[interface{}]interface{}{}
	uniqValues := make([]interface{}, 0, len(values))
	for _, val := range values {
		if _, found := valuesMap[val]; !found {
			uniqValues = append(uniqValues, val)
			valuesMap[val] = true
		}
	}

	q.appendFilterExpressionAndValues("filter-in", fmt.Sprint(column, " in (", strings.Trim(strings.Repeat("?,", len(uniqValues)), ","), ")"), uniqValues...)
	return q
}

func (q Query) prepareExpr(expr ...interface{}) (string, []interface{}) {
	query := utils.NewStringBuilder()
	var values []interface{}
	for _, part := range expr {
		if val, isVal := part.(exprValue); isVal {
			query.Append("?")
			values = append(values, val.val)
		} else if val, isVal := part.(string); isVal {
			query.Append(val)
		} else {
			query.Append(fmt.Sprint(val))
		}
		query.Append(" ")
	}
	return query.String(), values
}

func (q *Query) FilterExpr(expr ...interface{}) *Query {
	query, params := q.prepareExpr(expr...)
	return q.FilterRawExpression(query, params...)
}

// FulltextSearch uses postgresql fulltext, which means the words must be whole (i.e. it won't search by substrings)
func (q *Query) FulltextSearch(columns []string, values []string) *Query {
	var expr []interface{}
	expr = append(expr, "to_tsvector(concat(")
	for n, col := range columns {
		if n > 0 {
			expr = append(expr, ", ' ', ")
		}
		expr = append(expr, col)
	}
	expr = append(expr, ")) @@ ")
	expr = append(expr, "to_tsquery(")

	var cleanedValues []string
	for _, val := range values {
		for _, val2 := range strings.Split(val, "&") {
			val2 = strings.TrimSpace(val2)
			if val2 != "" {
				cleanedValues = append(cleanedValues, val2)
			}
		}
	}

	expr = append(expr, ExprValue(strings.Join(cleanedValues, " & ")))
	expr = append(expr, ")")
	return q.FilterExpr(expr...)
}

// FulltextSubsttringSearch can be slow!
func (q *Query) FulltextSubsttringSearch(table string, columns []string, values []string, limit int) (*QueryIterator, error) {
	query := ""
	var params []interface{}

	query += "select * from (select " + table + ".*, "
	for n, col := range columns {
		if n > 0 {
			query += " ||' '|| "
		}
		query += col
	}
	query += " search_string from " + table + ") as search_table where "
	for n, val := range values {
		if n > 0 {
			query += " and "
		}
		query += "search_string ilike '%'||?||'%' "
		params = append(params, val)
	}
	query += " limit " + fmt.Sprint(limit)
	return q.RawIterator(query, params...)
}

func (q *Query) WithPageNoString(s string) *Query {
	// TODO: Note errors ignored, keep them and then return in All() or Count() or ...
	n, _ := strconv.ParseInt(s, 10, 64)
	return q.WithPageNo(int(n))
}

func (q *Query) WithStringPageNo(n string) *Query {
	if n == "" {
		q.pageNo = 0
	} else {
		p, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			q.err = merry.Wrap(err).Appendf("invalid page: %s", n)
			return q
		}
		q.pageNo = int(p)
	}
	return q
}

func (q *Query) WithPageNo(n int) *Query {
	q.logStr = append(q.logStr, fmt.Sprintf("page:%d", n))
	q.pageNo = n
	return q
}

func (q *Query) WithMaxIntPageSize() *Query {
	return q.WithPageSize(math.MaxInt32)
}

func (q *Query) WithPageSize(s int) *Query {
	q.logStr = append(q.logStr, fmt.Sprintf("page-size:%d", s))
	q.pageSize = s
	return q
}

func (q *Query) WithDefaultPageSize() *Query {
	return q.WithPageSize(defaultPageSize)
}

func (q *Query) OrderByRawExpression(expression string, values ...interface{}) *Query {
	if strings.Count(expression, "?") != len(values) {
		q.err = merry.New("invalid expression placeholders count").Appendf("'%s' params: %#v", expression, values)
	}
	q.orderBy = append(q.orderBy, expression)
	q.values = append(q.values, values...)
	q.logStr = append(q.logStr, fmt.Sprint("order_by:", expression))
	return q
}

func (q *Query) OrderByExpression(expr ...interface{}) *Query {
	query, params := q.prepareExpr(expr...)
	q.OrderByRawExpression(query, params...)
	return q
}

func (q *Query) OrderByAsc(column string) *Query {
	return q.OrderByRawExpression(column + " asc")
}

func (q *Query) OrderByDesc(column string) *Query {
	return q.OrderByRawExpression(column + " desc")
}

func (q *Query) whereExpressionAndValues() []interface{} {
	var expr bytes.Buffer
	for _, e := range q.expressions {
		if expr.Len() > 0 {
			_, _ = expr.WriteString(" and ")
		}
		_, _ = expr.WriteString(" (" + e + ") ")
	}

	var res []interface{}
	res = append(res, expr.String())
	res = append(res, q.values...)
	return res
}

func (q *Query) getLogStr() string {
	return strings.Join(q.logStr, " ")
}

func (q *Query) Count(sample Model) (int, error) {
	if q.err != nil {
		return 0, q.err
	}

	q.logStr = append(q.logStr, fmt.Sprintf("count:%T", sample))

	where := q.whereExpressionAndValues()
	started := time.Now()
	defer q.statsCollector.AddStats(q.c, started, q.getLogStr())

	var count int
	if err := q.prepareDb().Model(sample).Where(where[0], where[1:]...).Count(&count).Error; err != nil {
		return 0, merry.Wrap(err).Appendf("counting %T", sample)
	}

	return count, nil
}

func (q *Query) First(target Model) error {
	if q.err != nil {
		return q.err
	}

	q.logStr = append(q.logStr, fmt.Sprintf("first:%T", target))

	where := q.whereExpressionAndValues()
	started := time.Now()
	defer q.statsCollector.AddStats(q.c, started, q.getLogStr())

	if err := q.prepareDb().First(target, where...).Error; err != nil {
		code := http.StatusInternalServerError
		if IsRecordNotFound(err) {
			code = http.StatusNotFound
		}
		return merry.Wrap(err).Appendf("error getting first %T", target).WithHTTPCode(code)
	}
	return nil
}

func (q *Query) AllWithPageFull(target interface{}) (pageFull bool, err error) {
	if q.pageSize == 0 {
		q.pageSize = defaultPageSize
	}
	if err := q.All(target); err != nil {
		return false, err
	}
	count := reflector.New(target).Len()
	//fmt.Printf("pageSize=%d, len=%d, target=%T\n", q.pageSize, count, target)
	return count >= q.pageSize, nil
}

func (q *Query) allDb(model Model, target interface{}) (*gorm.DB, error) {
	if q.err != nil {
		return nil, q.err
	}

	where := q.whereExpressionAndValues()

	if len(q.logStr) == 0 {
		q.logStr = append(q.logStr, fmt.Sprintf("all:%T", target))
	} else {
		q.logStr = append(q.logStr, fmt.Sprintf("%T", target))
	}

	started := time.Now()
	defer q.statsCollector.AddStats(q.c, started, q.getLogStr())

	if q.pageSize == 0 {
		q.pageSize = defaultPageSize
	}
	db := q.prepareDb()
	if q.pageNo > 0 {
		db = db.Offset(q.pageNo * q.pageSize)
	}

	if model == nil && target != nil {
		if reflect.TypeOf(target).Kind() != reflect.Ptr {
			return nil, merry.New("must be pointer").Appendf("found %T", target)
		}
		return db.Limit(q.pageSize).Find(target, where...), nil
	} else if model != nil && target == nil {
		return db.Limit(q.pageSize).Model(model).Where(where[0], where[1:]...), nil
	}
	return nil, merry.New("invalid model and target").Appendf("Model %T, target: %T", model, target)
}

func (q *Query) AllIterator(m Model) (*QueryIterator, error) {
	db, err := q.allDb(m, nil)
	if err != nil {
		return nil, err
	}
	rows, err := db.Rows()
	if err != nil {
		return nil, err
	}
	return &QueryIterator{rows: rows, db: db}, nil
}

func (q *Query) RawIterator(sql string, values ...interface{}) (*QueryIterator, error) {
	rows, err := q.RawRows(sql, values...)
	if err != nil {
		return nil, err
	}
	return &QueryIterator{rows: rows, db: q.gormDb}, nil
}

func (q *Query) All(target interface{}) error {
	db, err := q.allDb(nil, target)
	if err != nil {
		return err
	}
	if err := db.Error; err != nil && err != sql.ErrNoRows {
		return merry.Wrap(err).Appendf("getting %T", target)
	}
	return nil
}

func (q *Query) prepareDb() *gorm.DB {
	db := q.gormDb.New()
	for _, ord := range q.orderBy {
		db = db.Order(ord)
	}
	if q.includeDeleted {
		db = db.Unscoped()
	}
	return db
}
