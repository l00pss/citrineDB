package executor

import (
	"fmt"
	"time"

	"github.com/l00pss/citrinelexer"
)

// executeBegin handles BEGIN/BEGIN TRANSACTION statements
func (e *Executor) executeBegin(_ *citrinelexer.BeginStatement) (*Result, error) {
	if e.inTx {
		return nil, ErrTransactionActive
	}

	result := NewResult()

	if e.wal != nil {
		txID, err := e.wal.BeginTransaction(30 * time.Second)
		if err != nil {
			return nil, fmt.Errorf("executor: failed to begin transaction: %w", err)
		}
		e.activeTxID = txID
		e.inTx = true
		result.Message = fmt.Sprintf("Transaction started (ID: %s)", txID)
	} else {
		e.inTx = true
		result.Message = "Transaction started (WAL not configured)"
	}

	return result, nil
}

// executeCommit handles COMMIT statements
func (e *Executor) executeCommit(_ *citrinelexer.CommitStatement) (*Result, error) {
	if !e.inTx {
		return nil, ErrNoActiveTransaction
	}

	result := NewResult()

	if e.wal != nil && e.activeTxID != "" {
		indexes, err := e.wal.CommitTransaction(e.activeTxID)
		if err != nil {
			return nil, fmt.Errorf("executor: failed to commit transaction: %w", err)
		}
		result.Message = fmt.Sprintf("Transaction committed (%d entries written)", len(indexes))
		e.activeTxID = ""
	} else {
		result.Message = "Transaction committed"
	}

	e.inTx = false
	return result, nil
}

// executeRollback handles ROLLBACK statements
func (e *Executor) executeRollback(_ *citrinelexer.RollbackStatement) (*Result, error) {
	if !e.inTx {
		return nil, ErrNoActiveTransaction
	}

	result := NewResult()

	if e.wal != nil && e.activeTxID != "" {
		if err := e.wal.RollbackTransaction(e.activeTxID); err != nil {
			return nil, fmt.Errorf("executor: failed to rollback transaction: %w", err)
		}
		result.Message = "Transaction rolled back"
		e.activeTxID = ""
	} else {
		result.Message = "Transaction rolled back"
	}

	e.inTx = false
	return result, nil
}

// InTransaction returns whether a transaction is active
func (e *Executor) InTransaction() bool {
	return e.inTx
}

// logToWAL logs an operation to the WAL if in transaction
func (e *Executor) logToWAL(operation string, data []byte) error {
	if e.wal == nil || !e.inTx {
		return nil
	}

	logEntry := fmt.Sprintf("%s:%s", operation, string(data))
	return e.wal.AddToTransaction(e.activeTxID, []byte(logEntry))
}
