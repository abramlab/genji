// Package database provides database primitives such as tables, transactions and indexes.
package database

import (
	"context"
	"errors"
	"sync"

	"github.com/genjidb/genji/document/encoding"
	"github.com/genjidb/genji/engine"
)

const (
	InternalPrefix = "__genji_"
)

// A Database manages a list of tables in an engine.
type Database struct {
	ng engine.Engine

	// If this is non-nil, the user is running an explicit transaction
	// using the BEGIN statement.
	// Only one attached transaction can be run at a time and any calls to DB.Begin()
	// will cause an error until that transaction is rolled back or commited.
	attachedTransaction *Transaction
	attachedTxMu        sync.Mutex

	// Codec used to encode documents. Defaults to MessagePack.
	Codec encoding.Codec

	Catalog Catalog

	// This controls concurrency on read-only and read/write transactions.
	txmu sync.RWMutex
}

type Options struct {
	Codec   encoding.Codec
	Catalog Catalog
}

// New initializes the DB using the given engine.
func New(ctx context.Context, ng engine.Engine, opts Options) (*Database, error) {
	if opts.Codec == nil {
		return nil, errors.New("missing codec")
	}
	if opts.Catalog == nil {
		return nil, errors.New("missing catalog")
	}

	db := Database{
		ng:    ng,
		Codec: opts.Codec,
	}

	tx, err := db.BeginTx(ctx, &TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	db.Catalog = opts.Catalog
	tx.Catalog = db.Catalog

	err = db.Catalog.Load(tx)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &db, nil
}

// Close the underlying engine.
func (db *Database) Close() error {
	// If there is an attached transaction
	// it must be rolled back before closing the engine.
	if tx := db.GetAttachedTx(); tx != nil {
		_ = tx.Rollback()
	}
	db.txmu.Lock()
	defer db.txmu.Unlock()

	// release all sequences
	tx, err := db.beginTx(context.Background(), &TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Tx.Rollback()

	for _, seqName := range tx.Catalog.ListSequences() {
		seq, err := tx.Catalog.GetSequence(seqName)
		if err != nil {
			return err
		}

		err = seq.Release(tx)
		if err != nil {
			return err
		}
	}

	err = tx.Tx.Commit()
	if err != nil {
		return err
	}

	return db.ng.Close()
}

// Begin starts a new transaction with default options.
// The returned transaction must be closed either by calling Rollback or Commit.
func (db *Database) Begin(writable bool) (*Transaction, error) {
	return db.BeginTx(context.Background(), &TxOptions{
		ReadOnly: !writable,
	})
}

// BeginTx starts a new transaction with the given options.
// If opts is empty, it will use the default options.
// The returned transaction must be closed either by calling Rollback or Commit.
// If the Attached option is passed, it opens a database level transaction, which gets
// attached to the database and prevents any other transaction to be opened afterwards
// until it gets rolled back or commited.
func (db *Database) BeginTx(ctx context.Context, opts *TxOptions) (*Transaction, error) {
	if opts == nil {
		opts = new(TxOptions)
	}

	if !opts.ReadOnly {
		db.txmu.Lock()
	} else {
		db.txmu.RLock()
	}

	db.attachedTxMu.Lock()
	defer db.attachedTxMu.Unlock()

	if db.attachedTransaction != nil {
		return nil, errors.New("cannot open a transaction within a transaction")
	}

	return db.beginTx(ctx, opts)
}

// beginTx creates a transaction without locks.
func (db *Database) beginTx(ctx context.Context, opts *TxOptions) (*Transaction, error) {
	ntx, err := db.ng.Begin(ctx, engine.TxOptions{
		Writable: !opts.ReadOnly,
	})
	if err != nil {
		return nil, err
	}

	tx := Transaction{
		DB:       db,
		Tx:       ntx,
		Catalog:  db.Catalog,
		Writable: !opts.ReadOnly,
		attached: opts.Attached,
	}

	if opts.Attached {
		db.attachedTransaction = &tx
	}

	return &tx, nil
}

// TxOptions are passed to Begin to configure transactions.
type TxOptions struct {
	// Open a read-only transaction.
	ReadOnly bool
	// Set the transaction as global at the database level.
	// Any queries run by the database will use that transaction until it is
	// rolled back or commited.
	Attached bool
}

// GetAttachedTx returns the transaction attached to the database. It returns nil if there is no
// such transaction.
// The returned transaction is not thread safe.
func (db *Database) GetAttachedTx() *Transaction {
	db.attachedTxMu.Lock()
	defer db.attachedTxMu.Unlock()

	return db.attachedTransaction
}
