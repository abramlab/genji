package query_test

import (
	"testing"

	"github.com/asdine/genji/query"
	"github.com/asdine/genji/query/expr"
	"github.com/asdine/genji/query/q"
	"github.com/asdine/genji/record"
	"github.com/asdine/genji/value"
	"github.com/stretchr/testify/require"
)

func TestUpdateStatement(t *testing.T) {
	tests := []struct {
		name      string
		withIndex bool
	}{
		{"index", true},
		{"noindex", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx, cleanup := createTable(t, 10, test.withIndex)
			defer cleanup()

			res := query.Update("test").Set("age", expr.IntValue(20)).Where(q.IntField("age").Gt(20)).Exec(tx)
			require.NoError(t, res.Err())

			tb, err := tx.GetTable("test")
			require.NoError(t, err)

			st := record.NewStream(tb)
			count, err := st.Count()
			require.NoError(t, err)
			require.Equal(t, 10, count)

			err = st.Iterate(func(r record.Record) error {
				f, err := r.GetField("age")
				require.NoError(t, err)
				age, err := value.DecodeInt(f.Data)
				require.NoError(t, err)
				require.True(t, age <= 20)
				return nil
			})
			require.NoError(t, err)
		})
	}
}
