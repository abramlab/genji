package query

import (
	"database/sql/driver"

	"github.com/asdine/genji/database"
	"github.com/asdine/genji/query/expr"
	"github.com/asdine/genji/record"
)

// A Query can execute statements against the database. It can read or write data
// from any table, or even alter the structure of the database.
// Results are returned as streams.
type Query struct {
	Statements []Statement
}

// Run executes all the statements in their own transaction and returns the last result.
func (q Query) Run(db *database.DB, args []driver.NamedValue) Result {
	var res Result
	var tx *database.Tx
	var err error

	for _, stmt := range q.Statements {
		// it there is an opened transaction but there are still statements
		// to be executed, close the current transaction.
		if tx != nil {
			if tx.Writable() {
				err := tx.Commit()
				if err != nil {
					return Result{err: err}
				}
			} else {
				err := tx.Rollback()
				if err != nil {
					return Result{err: err}
				}
			}
		}

		// start a new transaction for every statement
		tx, err = db.Begin(!stmt.IsReadOnly())
		if err != nil {
			return Result{err: err}
		}

		res = stmt.Run(tx, args)
		if res.err != nil {
			tx.Rollback()
			return res
		}
	}

	// the returned result will now own the transaction.
	// its Close method is expected to be called.
	res.tx = tx

	return res
}

// Exec the query within the given transaction. If the one of the statements requires a read-write
// transaction and tx is not, tx will get promoted.
func (q Query) Exec(tx *database.Tx, args []driver.NamedValue, forceReadOnly bool) Result {
	var res Result

	for _, stmt := range q.Statements {
		// if the statement requires a writable transaction,
		// promote the current transaction.
		if !forceReadOnly && !tx.Writable() && !stmt.IsReadOnly() {
			err := tx.Promote()
			if err != nil {
				return Result{err: err}
			}
		}

		res = stmt.Run(tx, args)
		if res.err != nil {
			return res
		}
	}

	return res
}

// New creates a new Query with the given statements.
func New(statements ...Statement) Query {
	return Query{Statements: statements}
}

// A Statement represents a unique action that can be executed against the database.
type Statement interface {
	Run(*database.Tx, []driver.NamedValue) Result
	IsReadOnly() bool
}

// Result of a query.
type Result struct {
	record.Stream
	rowsAffected       driver.RowsAffected
	err                error
	lastInsertRecordID []byte
	tx                 *database.Tx
	closed             bool
}

// Err returns a non nil error if an error occured during the query.
func (r Result) Err() error {
	return r.err
}

// LastInsertId is not supported and returns an error.
// Use LastInsertRecordID instead.
func (r Result) LastInsertId() (int64, error) {
	return r.rowsAffected.LastInsertId()
}

// LastInsertRecordID returns the database's auto-generated recordID
// after, for example, an INSERT into a table with primary
// key.
func (r Result) LastInsertRecordID() ([]byte, error) {
	return r.lastInsertRecordID, nil
}

// RowsAffected returns the number of rows affected by the
// query.
func (r Result) RowsAffected() (int64, error) {
	return r.rowsAffected.RowsAffected()
}

// Close the result stream. It must be always be called when the
// result is not errored. Calling it when Err() is not nil is safe.
func (r *Result) Close() error {
	if r.closed {
		return nil
	}

	r.closed = true

	var err error

	if r.tx != nil {
		if r.tx.Writable() {
			err = r.tx.Commit()
		} else {
			err = r.tx.Rollback()
		}
	}

	return err
}

func whereClause(e expr.Expr, stack expr.EvalStack) func(r record.Record) (bool, error) {
	if e == nil {
		return func(r record.Record) (bool, error) {
			return true, nil
		}
	}

	return func(r record.Record) (bool, error) {
		stack.Record = r
		v, err := e.Eval(stack)
		if err != nil {
			return false, err
		}

		return v.Truthy(), nil
	}
}

func argsToNamedValues(args []interface{}) []driver.NamedValue {
	nv := make([]driver.NamedValue, len(args))
	for i := range args {
		switch t := args[i].(type) {
		case driver.NamedValue:
			nv[i] = t
		case *driver.NamedValue:
			nv[i] = *t
		default:
			nv[i].Ordinal = i + 1
			nv[i].Value = args[i]
		}
	}

	return nv
}

// A FieldSelector can extract a field from a record.
type FieldSelector interface {
	// SelectField takes a field from a record.
	// If the field selector was created using the As method
	// it must replace the name of f by the alias.
	SelectField(record.Record) (f record.Field, err error)
	// Name of the field selector.
	Name() string
	// As creates an alias to a field.
	// The returned field selector selects the same field but a different name
	// when the SelectField is called.
	As(alias string) FieldSelector
}

// TableSelector can select a table from a transaction.
type TableSelector interface {
	// SelectTable selects a table by calling the Table method of the transaction.
	SelectTable(*database.Tx) (record.Iterator, error)
	// Name of the selected table.
	TableName() string
}
