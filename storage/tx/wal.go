package tx

import (
	"time"

	"github.com/l00pss/walrus"
)

type WALManager struct {
	wal *walrus.WAL
	dir string
}

type WALConfig struct {
	Dir            string
	SegmentSize    int64
	MaxSegments    int
	SyncAfterWrite bool
	BufferSize     int
}

func DefaultWALConfig() WALConfig {
	return WALConfig{
		Dir:            "./wal",
		SegmentSize:    64 * 1024 * 1024,
		MaxSegments:    100,
		SyncAfterWrite: true,
		BufferSize:     4096,
	}
}

func NewWALManager(config WALConfig) (*WALManager, error) {
	walConfig := walrus.DefaultConfig()
	walResult := walrus.NewWAL(config.Dir, walConfig)
	if walResult.IsErr() {
		return nil, walResult.UnwrapErr()
	}
	return &WALManager{wal: walResult.Unwrap(), dir: config.Dir}, nil
}

func OpenWAL(config WALConfig) (*WALManager, error) {
	walConfig := walrus.DefaultConfig()
	walResult := walrus.Open(config.Dir, walConfig)
	if walResult.IsErr() {
		return nil, walResult.UnwrapErr()
	}
	return &WALManager{wal: walResult.Unwrap(), dir: config.Dir}, nil
}

func (w *WALManager) Append(data []byte) (uint64, error) {
	entry := walrus.Entry{Data: data, Timestamp: time.Now()}
	result := w.wal.Append(entry)
	if result.IsErr() {
		return 0, result.UnwrapErr()
	}
	return result.Unwrap(), nil
}

func (w *WALManager) Get(index uint64) ([]byte, error) {
	result := w.wal.Get(index)
	if result.IsErr() {
		return nil, result.UnwrapErr()
	}
	return result.Unwrap().Data, nil
}

func (w *WALManager) GetRange(start, end uint64) ([][]byte, error) {
	result := w.wal.GetRange(start, end)
	if result.IsErr() {
		return nil, result.UnwrapErr()
	}
	entries := result.Unwrap()
	data := make([][]byte, len(entries))
	for i, e := range entries {
		data[i] = e.Data
	}
	return data, nil
}

func (w *WALManager) WriteBatch(entries [][]byte) ([]uint64, error) {
	walEntries := make([]walrus.Entry, len(entries))
	for i, data := range entries {
		walEntries[i] = walrus.Entry{Data: data, Timestamp: time.Now()}
	}
	result := w.wal.WriteBatch(walEntries)
	if result.IsErr() {
		return nil, result.UnwrapErr()
	}
	return result.Unwrap(), nil
}

func (w *WALManager) BeginTransaction(timeout time.Duration) (walrus.TransactionID, error) {
	result := w.wal.BeginTransaction(timeout)
	if result.IsErr() {
		return "", result.UnwrapErr()
	}
	return result.Unwrap(), nil
}

func (w *WALManager) AddToTransaction(txID walrus.TransactionID, data []byte) error {
	entry := walrus.Entry{Data: data, Timestamp: time.Now()}
	result := w.wal.AddToTransaction(txID, entry)
	if result.IsErr() {
		return result.UnwrapErr()
	}
	return nil
}

func (w *WALManager) CommitTransaction(txID walrus.TransactionID) ([]uint64, error) {
	result := w.wal.CommitTransaction(txID)
	if result.IsErr() {
		return nil, result.UnwrapErr()
	}
	return result.Unwrap(), nil
}

func (w *WALManager) RollbackTransaction(txID walrus.TransactionID) error {
	result := w.wal.RollbackTransaction(txID)
	if result.IsErr() {
		return result.UnwrapErr()
	}
	return nil
}

func (w *WALManager) Truncate(index uint64) error {
	result := w.wal.Truncate(index)
	if result.IsErr() {
		return result.UnwrapErr()
	}
	return nil
}

func (w *WALManager) GetFirstIndex() uint64 { return w.wal.GetFirstIndex() }
func (w *WALManager) GetLastIndex() uint64  { return w.wal.GetLastIndex() }

func (w *WALManager) Close() error {
	return w.wal.Close()
}
