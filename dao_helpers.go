package dao

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

const defaultPageSize = 50

type queryExpression struct {
	expression string
	values     []interface{}
}

type Query struct {
	gormDb         *gorm.DB
	logger         Logger
	c              context.Context
	statsCollector DbStatsCollector

	pageSize       int
	includeDeleted bool
	logStr         bytes.Buffer
	filters        []queryExpression
	orderBy        []string
}

func (q *Query) IncludeDeleted() *Query {
	q.includeDeleted = true
	return q
}

func (q *Query) FilterExpression(expression string, values ...interface{}) *Query {
	q.filters = append(q.filters, queryExpression{
		expression: expression,
		values:     values,
	})
	_, _ = q.logStr.WriteRune(' ')
	_, _ = q.logStr.WriteString(expression)
	return q
}

func (q *Query) Filter(column, operation string, value interface{}) *Query {
	q.filters = append(q.filters, queryExpression{
		expression: column + operation + "?",
		values:     []interface{}{value},
	})
	_, _ = q.logStr.WriteRune(' ')
	_, _ = q.logStr.WriteString(column + operation)
	return q
}

func (q *Query) WithPageSize(s int) *Query {
	_, _ = q.logStr.WriteRune(' ')
	_, _ = q.logStr.WriteString(fmt.Sprintf("page:%d", s))
	q.pageSize = s
	return q
}

func (q *Query) WithDefaultPageSize() *Query {
	return q.WithPageSize(defaultPageSize)
}

func (q *Query) OrderBy(column string, ascDesc ...string) *Query {
	if len(ascDesc) == 0 {
		ascDesc = []string{"asc"}
	}
	_, _ = q.logStr.WriteRune(' ')
	_, _ = q.logStr.WriteString(fmt.Sprintf("order by:%s %s", column, ascDesc))
	q.orderBy = append(q.orderBy, column+" "+ascDesc[0])
	return q
}

func (q *Query) whereExpressionAndValues() []interface{} {
	var expr bytes.Buffer
	var params []interface{}
	for _, e := range q.filters {
		if expr.Len() > 0 {
			_, _ = expr.WriteString(" and ")
		}
		_, _ = expr.WriteString(" (" + e.expression + ") ")
		params = append(params, e.values)
	}

	var res []interface{}
	res = append(res, expr.String())
	res = append(res, params...)
	return res
}

func (q *Query) Count(sample Model) (int, error) {
	_, _ = q.logStr.WriteRune(' ')
	_, _ = q.logStr.WriteString(fmt.Sprintf("%T", sample))

	where := q.whereExpressionAndValues()
	started := time.Now()
	defer q.statsCollector.AddDbStats(q.c, started, q.logStr.String())

	var count int
	if err := q.prepareDb().Model(sample).Where(where[0], where[1:]...).Count(&count).Error; err != nil {
		return 0, errors.Wrapf(err, "counting %T", sample)
	}

	return count, nil
}

func (q *Query) First(target Model) error {
	_, _ = q.logStr.WriteRune(' ')
	_, _ = q.logStr.WriteString(fmt.Sprintf("%T", target))

	where := q.whereExpressionAndValues()
	started := time.Now()
	defer q.statsCollector.AddDbStats(q.c, started, q.logStr.String())

	if err := q.prepareDb().First(target, where...).Error; err != nil {
		return errors.Wrapf(err, "error getting first %T", target)
	}
	return nil
}

func (q *Query) All(target interface{}) error {
	if reflect.TypeOf(target).Kind() != reflect.Ptr {
		return errors.Errorf("must be pointer found %T", target)
	}

	where := q.whereExpressionAndValues()

	if q.logStr.Len() == 0 {
		q.logStr.WriteString(fmt.Sprintf("all into %T", target))
	} else {
		_, _ = q.logStr.WriteRune(' ')
		_, _ = q.logStr.WriteString(fmt.Sprintf("%T", target))
	}

	started := time.Now()
	defer q.statsCollector.AddDbStats(q.c, started, q.logStr.String())

	if q.pageSize == 0 {
		q.pageSize = defaultPageSize
	}
	if err := q.prepareDb().Limit(q.pageSize).Find(target, where...).Error; err != nil {
		return errors.Wrapf(err, "getting %T", target)
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
