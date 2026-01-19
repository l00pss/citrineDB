package tx

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewWALManager(t *testing.T) {
	dir := t.TempDir()
	walDir := filepath.Join(dir, "wal")

	wm, err := NewWALManager(WALConfig{Dir: walDir})
	if err != nil {
		t.Fatalf("NewWALManager: %v", err)
	}
	defer wm.Close()

	if wm.GetFirstIndex() != 0 {
		t.Errorf("FirstIndex: want 0, got %d", wm.GetFirstIndex())
	}
	if wm.GetLastIndex() != 0 {
		t.Errorf("LastIndex: want 0, got %d", wm.GetLastIndex())
	}
}

func TestAppendAndGet(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	data := []byte("test entry")
	index, err := wm.Append(data)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if index != 1 {
		t.Errorf("Append index: want 1, got %d", index)
	}

	got, err := wm.Get(index)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("Get: want %s, got %s", data, got)
	}
}

func TestWriteBatch(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	entries := [][]byte{
		[]byte("entry1"),
		[]byte("entry2"),
		[]byte("entry3"),
	}

	indices, err := wm.WriteBatch(entries)
	if err != nil {
		t.Fatalf("WriteBatch: %v", err)
	}
	if len(indices) != 3 {
		t.Errorf("WriteBatch indices: want 3, got %d", len(indices))
	}

	for i, idx := range indices {
		data, _ := wm.Get(idx)
		if string(data) != string(entries[i]) {
			t.Errorf("Entry %d: want %s, got %s", i, entries[i], data)
		}
	}
}

func TestGetRange(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	for i := 0; i < 10; i++ {
		wm.Append([]byte("entry"))
	}

	data, err := wm.GetRange(3, 7)
	if err != nil {
		t.Fatalf("GetRange: %v", err)
	}
	if len(data) != 5 {
		t.Errorf("GetRange: want 5, got %d", len(data))
	}
}

func TestTransaction(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	txID, err := wm.BeginTransaction(30 * time.Second)
	if err != nil {
		t.Fatalf("BeginTransaction: %v", err)
	}

	wm.AddToTransaction(txID, []byte("tx entry 1"))
	wm.AddToTransaction(txID, []byte("tx entry 2"))

	indices, err := wm.CommitTransaction(txID)
	if err != nil {
		t.Fatalf("CommitTransaction: %v", err)
	}
	if len(indices) != 2 {
		t.Errorf("Commit indices: want 2, got %d", len(indices))
	}
}

func TestRollbackTransaction(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	txID, _ := wm.BeginTransaction(30 * time.Second)
	wm.AddToTransaction(txID, []byte("will be rolled back"))

	if err := wm.RollbackTransaction(txID); err != nil {
		t.Fatalf("RollbackTransaction: %v", err)
	}

	if wm.GetLastIndex() != 0 {
		t.Errorf("LastIndex after rollback: want 0, got %d", wm.GetLastIndex())
	}
}

func TestRecovery(t *testing.T) {
	dir := t.TempDir()
	walDir := filepath.Join(dir, "wal")

	wm, _ := NewWALManager(WALConfig{Dir: walDir})
	wm.Append([]byte("persistent data"))
	wm.Close()

	wm2, err := OpenWAL(WALConfig{Dir: walDir})
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	defer wm2.Close()

	if wm2.GetLastIndex() != 1 {
		t.Errorf("Recovered LastIndex: want 1, got %d", wm2.GetLastIndex())
	}

	data, _ := wm2.Get(1)
	if string(data) != "persistent data" {
		t.Errorf("Recovered data: want 'persistent data', got %s", data)
	}
}

func TestTruncate(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	for i := 0; i < 10; i++ {
		wm.Append([]byte("entry"))
	}

	if err := wm.Truncate(5); err != nil {
		t.Fatalf("Truncate: %v", err)
	}

	if wm.GetLastIndex() != 5 {
		t.Errorf("LastIndex after truncate: want 5, got %d", wm.GetLastIndex())
	}
}

func TestDefaultWALConfig(t *testing.T) {
	config := DefaultWALConfig()
	if config.Dir != "./wal" {
		t.Errorf("Dir: want ./wal, got %s", config.Dir)
	}
	if config.SegmentSize != 64*1024*1024 {
		t.Errorf("SegmentSize: want 64MB, got %d", config.SegmentSize)
	}
	if config.MaxSegments != 100 {
		t.Errorf("MaxSegments: want 100, got %d", config.MaxSegments)
	}
	if !config.SyncAfterWrite {
		t.Error("SyncAfterWrite: want true")
	}
	if config.BufferSize != 4096 {
		t.Errorf("BufferSize: want 4096, got %d", config.BufferSize)
	}
}

func TestGetInvalidIndex(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	_, err := wm.Get(999)
	if err == nil {
		t.Error("Get invalid index should return error")
	}
}

func TestGetRangeInvalid(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	// GetRange on empty WAL should return empty slice (not panic)
	entries, err := wm.GetRange(10, 20)
	if err != nil {
		t.Errorf("GetRange on empty WAL returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("GetRange on empty WAL should return empty slice, got %d entries", len(entries))
	}
}

func TestMultipleAppends(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	for i := 0; i < 100; i++ {
		idx, err := wm.Append([]byte("entry"))
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		if idx != uint64(i+1) {
			t.Errorf("Append %d: want index %d, got %d", i, i+1, idx)
		}
	}

	if wm.GetFirstIndex() != 1 {
		t.Errorf("FirstIndex: want 1, got %d", wm.GetFirstIndex())
	}
	if wm.GetLastIndex() != 100 {
		t.Errorf("LastIndex: want 100, got %d", wm.GetLastIndex())
	}
}

func TestEmptyWriteBatch(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	indices, err := wm.WriteBatch([][]byte{})
	if err != nil {
		t.Fatalf("Empty WriteBatch: %v", err)
	}
	if len(indices) != 0 {
		t.Errorf("Empty WriteBatch: want 0 indices, got %d", len(indices))
	}
}

func TestTransactionCommitVerifyData(t *testing.T) {
	dir := t.TempDir()
	wm, _ := NewWALManager(WALConfig{Dir: filepath.Join(dir, "wal")})
	defer wm.Close()

	txID, _ := wm.BeginTransaction(30 * time.Second)
	wm.AddToTransaction(txID, []byte("tx data 1"))
	wm.AddToTransaction(txID, []byte("tx data 2"))
	indices, _ := wm.CommitTransaction(txID)

	data1, _ := wm.Get(indices[0])
	data2, _ := wm.Get(indices[1])

	if string(data1) != "tx data 1" {
		t.Errorf("TX data 1: want 'tx data 1', got %s", data1)
	}
	if string(data2) != "tx data 2" {
		t.Errorf("TX data 2: want 'tx data 2', got %s", data2)
	}
}
