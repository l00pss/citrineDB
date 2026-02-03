package engine

import (
	"os"
	"testing"
)

func TestOpenClose(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	defer os.RemoveAll(dbPath)
	defer os.RemoveAll(dbPath + ".wal")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if db.closed {
		t.Error("database should be open")
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("failed to close database: %v", err)
	}

	if !db.closed {
		t.Error("database should be closed")
	}
}

func TestCreateTableAndStats(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	defer os.RemoveAll(dbPath)
	defer os.RemoveAll(dbPath + ".wal")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	result, err := db.Execute("CREATE TABLE users (id INTEGER, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	if result.Message == "" {
		t.Error("expected success message")
	}

	stats := db.Stats()
	if stats.TableCount != 1 {
		t.Errorf("expected 1 table, got %d", stats.TableCount)
	}
}

func TestStats(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	defer os.RemoveAll(dbPath)
	defer os.RemoveAll(dbPath + ".wal")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	stats := db.Stats()
	if stats.PageSize != 4096 {
		t.Errorf("expected page size 4096, got %d", stats.PageSize)
	}

	if stats.BufferPoolSize != 1024 {
		t.Errorf("expected buffer pool size 1024, got %d", stats.BufferPoolSize)
	}
}

func TestDoubleClose(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	defer os.RemoveAll(dbPath)
	defer os.RemoveAll(dbPath + ".wal")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("failed to close database: %v", err)
	}

	err = db.Close()
	if err != ErrDatabaseClosed {
		t.Errorf("expected ErrDatabaseClosed, got %v", err)
	}
}

func TestExecuteOnClosedDB(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	defer os.RemoveAll(dbPath)
	defer os.RemoveAll(dbPath + ".wal")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	db.Close()

	_, err = db.Execute("CREATE TABLE test (id INTEGER)")
	if err != ErrDatabaseClosed {
		t.Errorf("expected ErrDatabaseClosed, got %v", err)
	}
}

func TestRowsInterface(t *testing.T) {
	rows := &Rows{
		columns: []string{"id", "name"},
		rows: [][]interface{}{
			{int32(1), "Alice"},
			{int32(2), "Bob"},
		},
		index: -1,
	}

	cols := rows.Columns()
	if len(cols) != 2 {
		t.Errorf("expected 2 columns, got %d", len(cols))
	}

	count := 0
	for rows.Next() {
		count++
		row := rows.Row()
		if row == nil {
			t.Error("row should not be nil")
		}
	}

	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}

	if rows.Count() != 2 {
		t.Errorf("expected count 2, got %d", rows.Count())
	}
}

func TestScanValue(t *testing.T) {
	var i int
	var i32 int32
	var i64 int64
	var f64 float64
	var s string
	var b bool

	// Test int scan
	err := scanValue(&i, int32(42))
	if err != nil {
		t.Errorf("failed to scan int: %v", err)
	}
	if i != 42 {
		t.Errorf("expected 42, got %d", i)
	}

	// Test int32 scan
	err = scanValue(&i32, int32(100))
	if err != nil {
		t.Errorf("failed to scan int32: %v", err)
	}
	if i32 != 100 {
		t.Errorf("expected 100, got %d", i32)
	}

	// Test int64 scan
	err = scanValue(&i64, int64(1000))
	if err != nil {
		t.Errorf("failed to scan int64: %v", err)
	}
	if i64 != 1000 {
		t.Errorf("expected 1000, got %d", i64)
	}

	// Test float64 scan
	err = scanValue(&f64, float64(3.14))
	if err != nil {
		t.Errorf("failed to scan float64: %v", err)
	}
	if f64 != 3.14 {
		t.Errorf("expected 3.14, got %f", f64)
	}

	// Test string scan
	err = scanValue(&s, "hello")
	if err != nil {
		t.Errorf("failed to scan string: %v", err)
	}
	if s != "hello" {
		t.Errorf("expected 'hello', got '%s'", s)
	}

	// Test bool scan
	err = scanValue(&b, true)
	if err != nil {
		t.Errorf("failed to scan bool: %v", err)
	}
	if !b {
		t.Error("expected true")
	}
}
