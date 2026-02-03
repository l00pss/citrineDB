package tx

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/l00pss/walrus"
)

var (
	ErrTxNotStarted   = errors.New("tx: transaction not started")
	ErrTxAlreadyEnded = errors.New("tx: transaction already ended")
	ErrTxConflict     = errors.New("tx: transaction conflict detected")
	ErrDeadlock       = errors.New("tx: deadlock detected")
)

type TxID uint64

type TxStatus int

const (
	TxStatusActive TxStatus = iota
	TxStatusCommitted
	TxStatusAborted
)

func (s TxStatus) String() string {
	switch s {
	case TxStatusActive:
		return "active"
	case TxStatusCommitted:
		return "committed"
	case TxStatusAborted:
		return "aborted"
	default:
		return "unknown"
	}
}

type IsolationLevel int

const (
	ReadUncommitted IsolationLevel = iota
	ReadCommitted
	RepeatableRead
	Serializable
)

func (l IsolationLevel) String() string {
	switch l {
	case ReadUncommitted:
		return "READ UNCOMMITTED"
	case ReadCommitted:
		return "READ COMMITTED"
	case RepeatableRead:
		return "REPEATABLE READ"
	case Serializable:
		return "SERIALIZABLE"
	default:
		return "UNKNOWN"
	}
}

type Transaction struct {
	ID             TxID
	Status         TxStatus
	IsolationLevel IsolationLevel
	StartTime      time.Time
	Savepoints     []string
	ReadSet        map[string]uint64
	WriteSet       map[string]interface{}
	walTxID        walrus.TransactionID
	mu             sync.RWMutex
}

func NewTransaction(id TxID, isolation IsolationLevel) *Transaction {
	return &Transaction{
		ID:             id,
		Status:         TxStatusActive,
		IsolationLevel: isolation,
		StartTime:      time.Now(),
		Savepoints:     make([]string, 0),
		ReadSet:        make(map[string]uint64),
		WriteSet:       make(map[string]interface{}),
	}
}

func (tx *Transaction) IsActive() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.Status == TxStatusActive
}

func (tx *Transaction) AddToReadSet(key string, version uint64) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.ReadSet[key] = version
}

func (tx *Transaction) AddToWriteSet(key string, value interface{}) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.WriteSet[key] = value
}

func (tx *Transaction) CreateSavepoint(name string) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.Savepoints = append(tx.Savepoints, name)
}

func (tx *Transaction) RollbackToSavepoint(name string) bool {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	for i := len(tx.Savepoints) - 1; i >= 0; i-- {
		if tx.Savepoints[i] == name {
			tx.Savepoints = tx.Savepoints[:i]
			return true
		}
	}
	return false
}

type TxManager struct {
	mu                 sync.RWMutex
	nextTxID           uint64
	activeTransactions map[TxID]*Transaction
	defaultIsolation   IsolationLevel
	wal                *WALManager
}

func NewTxManager(wal *WALManager) *TxManager {
	return &TxManager{
		activeTransactions: make(map[TxID]*Transaction),
		defaultIsolation:   ReadCommitted,
		wal:                wal,
	}
}

func (m *TxManager) Begin() *Transaction {
	return m.BeginWithIsolation(m.defaultIsolation)
}

func (m *TxManager) BeginWithIsolation(isolation IsolationLevel) *Transaction {
	id := TxID(atomic.AddUint64(&m.nextTxID, 1))
	tx := NewTransaction(id, isolation)

	m.mu.Lock()
	m.activeTransactions[id] = tx
	m.mu.Unlock()

	if m.wal != nil {
		walTxID, err := m.wal.BeginTransaction(30 * time.Second)
		if err == nil {
			tx.walTxID = walTxID
		}
	}

	return tx
}

func (m *TxManager) Commit(tx *Transaction) error {
	if tx == nil {
		return ErrTxNotStarted
	}

	tx.mu.Lock()
	if tx.Status != TxStatusActive {
		tx.mu.Unlock()
		return ErrTxAlreadyEnded
	}

	if err := m.validateTransaction(tx); err != nil {
		tx.Status = TxStatusAborted
		tx.mu.Unlock()
		m.removeTransaction(tx.ID)
		return err
	}

	tx.Status = TxStatusCommitted
	tx.mu.Unlock()

	if m.wal != nil && tx.walTxID != "" {
		m.wal.CommitTransaction(tx.walTxID)
	}

	m.removeTransaction(tx.ID)
	return nil
}

func (m *TxManager) Rollback(tx *Transaction) error {
	if tx == nil {
		return ErrTxNotStarted
	}

	tx.mu.Lock()
	if tx.Status != TxStatusActive {
		tx.mu.Unlock()
		return ErrTxAlreadyEnded
	}

	tx.Status = TxStatusAborted
	tx.mu.Unlock()

	if m.wal != nil && tx.walTxID != "" {
		m.wal.RollbackTransaction(tx.walTxID)
	}

	m.removeTransaction(tx.ID)
	return nil
}

func (m *TxManager) removeTransaction(id TxID) {
	m.mu.Lock()
	delete(m.activeTransactions, id)
	m.mu.Unlock()
}

func (m *TxManager) validateTransaction(tx *Transaction) error {
	if tx.IsolationLevel < RepeatableRead {
		return nil
	}
	return nil
}

func (m *TxManager) GetTransaction(id TxID) *Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeTransactions[id]
}

func (m *TxManager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.activeTransactions)
}

func (m *TxManager) SetDefaultIsolation(level IsolationLevel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultIsolation = level
}
