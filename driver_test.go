package genji

import (
	"context"
	"database/sql"
	"testing"

	"github.com/asdine/genji/engine"
	"github.com/stretchr/testify/require"
)

type rectest struct {
	A int
	B []int
	C struct{ Foo string }
}

type foo struct{ Foo string }

func TestDriver(t *testing.T) {
	db, err := sql.Open("genji", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	res, err := db.Exec("CREATE TABLE test")
	require.NoError(t, err)
	n, err := res.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 0, n)

	for i := 0; i < 10; i++ {
		res, err = db.Exec("INSERT INTO test (a, b, c) VALUES (?, ?, ?)", i, []int{i + 1, i + 2, i + 3}, &foo{Foo: "bar"})
		require.NoError(t, err)
		n, err = res.RowsAffected()
		require.NoError(t, err)
		require.EqualValues(t, 1, n)
	}

	t.Run("Wildcard", func(t *testing.T) {
		rows, err := db.Query("SELECT * FROM test")
		require.NoError(t, err)
		defer rows.Close()

		var count int
		var rt rectest
		for rows.Next() {
			err = rows.Scan(Scanner(&rt))
			require.NoError(t, err)
			require.Equal(t, rectest{count, []int{count + 1, count + 2, count + 3}, foo{Foo: "bar"}}, rt)
			count++
		}

		require.NoError(t, rows.Err())
		require.Equal(t, 10, count)
	})

	t.Run("Multiple fields", func(t *testing.T) {
		rows, err := db.Query("SELECT a, c FROM test")
		require.NoError(t, err)
		defer rows.Close()

		var count int
		var a int
		var c foo
		for rows.Next() {
			err = rows.Scan(&a, Scanner(&c))
			require.NoError(t, err)
			require.Equal(t, count, a)
			require.Equal(t, foo{Foo: "bar"}, c)
			count++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 10, count)
	})

	t.Run("Multiple fields and wildcards", func(t *testing.T) {
		rows, err := db.Query("SELECT a, *, c, * FROM test")
		require.NoError(t, err)
		defer rows.Close()

		var count int
		var a int
		var c foo
		var rt1, rt2 rectest
		for rows.Next() {
			err = rows.Scan(&a, Scanner(&rt1), Scanner(&c), Scanner(&rt2))
			require.NoError(t, err)
			require.Equal(t, count, a)
			require.Equal(t, foo{Foo: "bar"}, c)
			require.Equal(t, rectest{count, []int{count + 1, count + 2, count + 3}, foo{Foo: "bar"}}, rt1)
			require.Equal(t, rectest{count, []int{count + 1, count + 2, count + 3}, foo{Foo: "bar"}}, rt2)
			count++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 10, count)
	})

	t.Run("Params", func(t *testing.T) {
		rows, err := db.Query("SELECT a FROM test WHERE a = ? AND b = ?", 5, []int{6, 7, 8})
		require.NoError(t, err)
		defer rows.Close()

		var count int
		var a int
		for rows.Next() {
			err = rows.Scan(&a)
			require.NoError(t, err)
			require.Equal(t, 5, a)
			count++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 1, count)
	})

	t.Run("Named Params", func(t *testing.T) {
		rows, err := db.Query("SELECT a FROM test WHERE a = $val", sql.Named("val", 5))
		require.NoError(t, err)
		defer rows.Close()

		var count int
		var a int
		for rows.Next() {
			err = rows.Scan(&a)
			require.NoError(t, err)
			require.Equal(t, 5, a)
			count++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 1, count)
	})

	t.Run("Transactions", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		rows, err := tx.Query("SELECT * FROM test")
		require.NoError(t, err)
		defer rows.Close()

		var count int
		var rt rectest
		for rows.Next() {
			err = rows.Scan(Scanner(&rt))
			require.NoError(t, err)
			require.Equal(t, rectest{count, []int{count + 1, count + 2, count + 3}, foo{Foo: "bar"}}, rt)
			count++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 10, count)
	})

	t.Run("Multiple queries", func(t *testing.T) {
		rows, err := db.Query(`
			SELECT * FROM test;;;
			INSERT INTO test (a, b, c) VALUES (10, [11, 12, 13], {foo: "bar"});
			SELECT * FROM test;
		`)
		require.NoError(t, err)
		defer rows.Close()

		var count int
		var rt rectest
		for rows.Next() {
			err = rows.Scan(Scanner(&rt))
			require.NoError(t, err)
			require.Equal(t, rectest{count, []int{count + 1, count + 2, count + 3}, foo{Foo: "bar"}}, rt)
			count++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 11, count)
	})

	t.Run("Multiple queries in transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		rows, err := tx.Query(`
			SELECT * FROM test;;;
			INSERT INTO test (a, b, c) VALUES (11, [12, 13, 14], {foo: "bar"});
			SELECT * FROM test;
		`)
		require.NoError(t, err)
		defer rows.Close()

		var count int
		var rt rectest
		for rows.Next() {
			err = rows.Scan(Scanner(&rt))
			require.NoError(t, err)
			require.Equal(t, rectest{count, []int{count + 1, count + 2, count + 3}, foo{Foo: "bar"}}, rt)
			count++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 12, count)
	})

	t.Run("Multiple queries in read only transaction", func(t *testing.T) {
		tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
		require.NoError(t, err)
		defer tx.Rollback()

		_, err = tx.Query(`
			SELECT * FROM test;;;
			INSERT INTO test (a, b, c) VALUES (12, 13, 14);
			SELECT * FROM test;
		`)
		require.Equal(t, err, engine.ErrTransactionReadOnly)
	})
}
