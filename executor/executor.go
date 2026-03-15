package executor

import (
	"errors"
	"fmt"

	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/tx"
	"github.com/l00pss/citrinelexer"
	"github.com/l00pss/walrus"
)

var (
	ErrUnsupportedStatement = errors.New("executor: unsupported statement type")
	ErrInvalidExpression    = errors.New("executor: invalid expression")
	ErrTypeMismatch         = errors.New("executor: type mismatch")
	ErrNullConstraint       = errors.New("executor: null constraint violation")
	ErrInvalidDataType      = errors.New("executor: invalid data type")
	ErrNoActiveTransaction  = errors.New("executor: no active transaction")
	ErrTransactionActive    = errors.New("executor: transaction already active")
)

// Result represents the result of a query execution
type Result struct {
	Columns      []string
	Rows         [][]interface{}
	RowsAffected int64
	LastInsertID int64
	Message      string
}

// NewResult creates a new empty result
func NewResult() *Result {
	return &Result{
		Columns: make([]string, 0),
		Rows:    make([][]interface{}, 0),
	}
}

// Executor handles SQL statement execution
type Executor struct {
	catalog    *catalog.Catalog
	wal        *tx.WALManager
	activeTxID walrus.TransactionID
	inTx       bool
}

// NewExecutor creates a new executor
func NewExecutor(cat *catalog.Catalog) *Executor {
	return &Executor{
		catalog: cat,
	}
}

// NewExecutorWithWAL creates a new executor with WAL support
func NewExecutorWithWAL(cat *catalog.Catalog, wal *tx.WALManager) *Executor {
	return &Executor{
		catalog: cat,
		wal:     wal,
	}
}

// Execute parses and executes a SQL statement
func (e *Executor) Execute(sql string) (*Result, error) {
	stmt, err := citrinelexer.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("executor: parse error: %w", err)
	}

	return e.ExecuteStatement(stmt)
}

// ExecuteStatement executes a parsed statement
func (e *Executor) ExecuteStatement(stmt citrinelexer.Statement) (*Result, error) {
	switch s := stmt.(type) {
	case *citrinelexer.CreateTableStatement:
		return e.executeCreateTable(s)
	case *citrinelexer.CreateIndexStatement:
		return e.executeCreateIndex(s)
	case *citrinelexer.DropIndexStatement:
		return e.executeDropIndex(s)
	case *citrinelexer.SelectStatement:
		return e.executeSelect(s)
	case *citrinelexer.InsertStatement:
		return e.executeInsert(s)
	case *citrinelexer.UpdateStatement:
		return e.executeUpdate(s)
	case *citrinelexer.DeleteStatement:
		return e.executeDelete(s)
	case *citrinelexer.BeginStatement:
		return e.executeBegin(s)
	case *citrinelexer.CommitStatement:
		return e.executeCommit(s)
	case *citrinelexer.RollbackStatement:
		return e.executeRollback(s)
	default:
		return nil, ErrUnsupportedStatement
	}
}
