package tx

import "testing"

func TestNewTransaction(t *testing.T) {
	tx := NewTransaction(1, ReadCommitted)
	if tx == nil {
		t.Fatal("expected transaction")
	}
	if tx.ID != 1 {
		t.Errorf("expected ID 1, got %d", tx.ID)
	}
	if tx.Status != TxStatusActive {
		t.Error("expected active status")
	}
	if tx.IsolationLevel != ReadCommitted {
		t.Error("expected ReadCommitted")
	}
}

func TestTxStatusString(t *testing.T) {
	if TxStatusActive.String() != "active" {
		t.Error("wrong string for active")
	}
	if TxStatusCommitted.String() != "committed" {
		t.Error("wrong string for committed")
	}
	if TxStatusAborted.String() != "aborted" {
		t.Error("wrong string for aborted")
	}
}

func TestIsolationLevelString(t *testing.T) {
	if ReadUncommitted.String() != "READ UNCOMMITTED" {
		t.Error("wrong string")
	}
	if ReadCommitted.String() != "READ COMMITTED" {
		t.Error("wrong string")
	}
	if RepeatableRead.String() != "REPEATABLE READ" {
		t.Error("wrong string")
	}
	if Serializable.String() != "SERIALIZABLE" {
		t.Error("wrong string")
	}
}

func TestTransactionIsActive(t *testing.T) {
	tx := NewTransaction(1, ReadCommitted)
	if !tx.IsActive() {
		t.Error("expected active")
	}
	tx.Status = TxStatusCommitted
	if tx.IsActive() {
		t.Error("expected not active")
	}
}

func TestTransactionReadWriteSet(t *testing.T) {
	tx := NewTransaction(1, ReadCommitted)
	tx.AddToReadSet("key1", 1)
	tx.AddToWriteSet("key2", "value")

	if tx.ReadSet["key1"] != 1 {
		t.Error("wrong read set value")
	}
	if tx.WriteSet["key2"] != "value" {
		t.Error("wrong write set value")
	}
}

func TestSavepoints(t *testing.T) {
	tx := NewTransaction(1, ReadCommitted)
	tx.CreateSavepoint("sp1")
	tx.CreateSavepoint("sp2")

	if len(tx.Savepoints) != 2 {
		t.Errorf("expected 2, got %d", len(tx.Savepoints))
	}

	if !tx.RollbackToSavepoint("sp1") {
		t.Error("expected rollback to succeed")
	}

	if len(tx.Savepoints) != 0 {
		t.Errorf("expected 0, got %d", len(tx.Savepoints))
	}

	if tx.RollbackToSavepoint("sp3") {
		t.Error("expected rollback to fail")
	}
}

func TestTxManager(t *testing.T) {
	mgr := NewTxManager(nil)
	if mgr == nil {
		t.Fatal("expected manager")
	}

	tx := mgr.Begin()
	if tx == nil {
		t.Fatal("expected transaction")
	}
	if tx.ID != 1 {
		t.Errorf("expected ID 1, got %d", tx.ID)
	}
	if mgr.ActiveCount() != 1 {
		t.Error("expected 1 active")
	}

	tx2 := mgr.BeginWithIsolation(Serializable)
	if tx2.IsolationLevel != Serializable {
		t.Error("wrong isolation level")
	}
	if mgr.ActiveCount() != 2 {
		t.Error("expected 2 active")
	}
}

func TestTxManagerCommit(t *testing.T) {
	mgr := NewTxManager(nil)
	tx := mgr.Begin()

	err := mgr.Commit(tx)
	if err != nil {
		t.Errorf("commit failed: %v", err)
	}
	if tx.Status != TxStatusCommitted {
		t.Error("expected committed")
	}
	if mgr.ActiveCount() != 0 {
		t.Error("expected 0 active")
	}

	err = mgr.Commit(tx)
	if err != ErrTxAlreadyEnded {
		t.Error("expected ErrTxAlreadyEnded")
	}
}

func TestTxManagerRollback(t *testing.T) {
	mgr := NewTxManager(nil)
	tx := mgr.Begin()

	err := mgr.Rollback(tx)
	if err != nil {
		t.Errorf("rollback failed: %v", err)
	}
	if tx.Status != TxStatusAborted {
		t.Error("expected aborted")
	}
	if mgr.ActiveCount() != 0 {
		t.Error("expected 0 active")
	}

	err = mgr.Rollback(tx)
	if err != ErrTxAlreadyEnded {
		t.Error("expected ErrTxAlreadyEnded")
	}
}

func TestTxManagerErrors(t *testing.T) {
	mgr := NewTxManager(nil)

	if err := mgr.Commit(nil); err != ErrTxNotStarted {
		t.Error("expected ErrTxNotStarted")
	}
	if err := mgr.Rollback(nil); err != ErrTxNotStarted {
		t.Error("expected ErrTxNotStarted")
	}
}

func TestGetTransaction(t *testing.T) {
	mgr := NewTxManager(nil)
	tx := mgr.Begin()

	got := mgr.GetTransaction(tx.ID)
	if got != tx {
		t.Error("expected same transaction")
	}

	got2 := mgr.GetTransaction(999)
	if got2 != nil {
		t.Error("expected nil for unknown ID")
	}
}

func TestSetDefaultIsolation(t *testing.T) {
	mgr := NewTxManager(nil)
	mgr.SetDefaultIsolation(Serializable)

	tx := mgr.Begin()
	if tx.IsolationLevel != Serializable {
		t.Error("expected Serializable")
	}
}
