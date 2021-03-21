package sq

import (
    "testing"

    "github.com/matryer/is"
)

func TestValuesQuery_ToSQL(t *testing.T) {
    type TT struct {
        description string
        q ValuesQuery
        wantQuery string
        wantArgs []interface{}
    }
    tests := []TT{
        {
            "Values",
            Values().
                Values("aaa", "aaa@example.com").
                Values("bbb", "bbb@example.com"),
            "VALUES ($1, $2), ($3, $4)",
            []interface{}{"aaa", "aaa@example.com", "bbb", "bbb@example.com"},
        },
    }
    for _, tt := range tests {
        tt := tt
        t.Run(tt.description, func(t *testing.T) {
            t.Parallel()
            is := is.New(t)
            var _ Query = tt.q
            gotQuery, gotArgs := tt.q.ToSQL()
            is.Equal(tt.wantQuery, gotQuery)
            is.Equal(tt.wantArgs, gotArgs)
        })
    }
}
