package sq

import (
    "strings"
)

// ValuesQuery represents a VALUES query.
type ValuesQuery struct {
    nested bool
    // VALUES
    RowValues RowValues
    // DB
    ColumnMapper func(*Column)
}

// Values returns a new empty ValuesQuery.
func Values() ValuesQuery {
    return ValuesQuery{}
}

// ToSQL marshals the ValuesQuery into a query string and args slice.
func (q ValuesQuery) ToSQL() (query string, args []interface{}) {
    defer func() {
        if r := recover(); r != nil {
            args = []interface{}{r}
        }
    }()
    buf := &strings.Builder{}
    q.AppendSQL(buf, &args, nil)
    return buf.String(), args
}

// AppendSQL marshals the ValuesQuery into a buffer and args slice. Do not call
// this as an end user, use ToSQL instead. AppendSQL may panic if you wrote
// panic code in your ColumnMapper, it is only exported to satisfy the Query
// interface.
func (q ValuesQuery) AppendSQL(buf *strings.Builder, args *[]interface{}, params map[string]int) {
    if q.ColumnMapper != nil {
        col := &Column{mode: colmodeInsert}
        q.ColumnMapper(col)
        q.RowValues = col.rowValues
    }
    // VALUES
    buf.WriteString("VALUES ")
    q.RowValues.AppendSQL(buf, args, nil)
    if !q.nested {
        query := buf.String()
        buf.Reset()
        questionToDollarPlaceholders(buf, query)
    }
}

// Values appends a new RowValue to the ValuesQuery.
func (q ValuesQuery) Values(values ...interface{}) ValuesQuery {
    q.RowValues = append(q.RowValues, values)
    return q
}

// Valuesx sets the column mapper for the ValuesQuery.
func (q ValuesQuery) Valuesx(mapper func(*Column)) ValuesQuery {
    q.ColumnMapper = mapper
    return q
}

// NestThis indicates to the ValuesQuery that it is nested.
func (q ValuesQuery) NestThis() Query {
    q.nested = true
    return q
}
