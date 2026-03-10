package executor

import (
	"testing"

	"github.com/l00pss/citrinedb/storage/buffer"
	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/file"
)

func setupDistinctTestExecutor(t *testing.T) (*Executor, func()) {
	t.Helper()

	dm, err := file.NewDiskManager(":memory:", file.Config{PageSize: 4096})
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}

	bp := buffer.NewBufferPool(dm, buffer.Config{PoolSize: 100})
	cat := catalog.NewCatalog(bp)
	if err := cat.Initialize(); err != nil {
		t.Fatalf("failed to initialize catalog: %v", err)
	}

	exec := NewExecutor(cat)

	cleanup := func() {
		dm.Close()
	}

	return exec, cleanup
}

func TestDistinctBasic(t *testing.T) {
	exec, cleanup := setupDistinctTestExecutor(t)
	defer cleanup()

	// Create table with duplicate values
	_, err := exec.Execute("CREATE TABLE colors (name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data with duplicates
	_, _ = exec.Execute("INSERT INTO colors VALUES ('red')")
	_, _ = exec.Execute("INSERT INTO colors VALUES ('blue')")
	_, _ = exec.Execute("INSERT INTO colors VALUES ('red')")
	_, _ = exec.Execute("INSERT INTO colors VALUES ('green')")
	_, _ = exec.Execute("INSERT INTO colors VALUES ('blue')")
	_, _ = exec.Execute("INSERT INTO colors VALUES ('red')")

	// Without DISTINCT - should have 6 rows
	result, err := exec.Execute("SELECT name FROM colors")
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}
	if len(result.Rows) != 6 {
		t.Errorf("expected 6 rows without DISTINCT, got %d", len(result.Rows))
	}

	// With DISTINCT - should have 3 unique rows
	result, err = exec.Execute("SELECT DISTINCT name FROM colors")
	if err != nil {
		t.Fatalf("SELECT DISTINCT failed: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows with DISTINCT, got %d", len(result.Rows))
	}
}

func TestDistinctMultipleColumns(t *testing.T) {
	exec, cleanup := setupDistinctTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE orders (customer TEXT, product TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data with duplicate combinations
	_, _ = exec.Execute("INSERT INTO orders VALUES ('Alice', 'Apple')")
	_, _ = exec.Execute("INSERT INTO orders VALUES ('Alice', 'Banana')")
	_, _ = exec.Execute("INSERT INTO orders VALUES ('Alice', 'Apple')") // Duplicate
	_, _ = exec.Execute("INSERT INTO orders VALUES ('Bob', 'Apple')")
	_, _ = exec.Execute("INSERT INTO orders VALUES ('Bob', 'Apple')") // Duplicate

	// With DISTINCT on multiple columns
	result, err := exec.Execute("SELECT DISTINCT customer, product FROM orders")
	if err != nil {
		t.Fatalf("SELECT DISTINCT failed: %v", err)
	}

	// Should have 3 unique combinations: (Alice,Apple), (Alice,Banana), (Bob,Apple)
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 unique combinations, got %d", len(result.Rows))
	}
}

func TestDistinctWithNulls(t *testing.T) {
	exec, cleanup := setupDistinctTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE data (value TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data with NULLs
	_, _ = exec.Execute("INSERT INTO data VALUES ('a')")
	_, _ = exec.Execute("INSERT INTO data VALUES (NULL)")
	_, _ = exec.Execute("INSERT INTO data VALUES ('a')")
	_, _ = exec.Execute("INSERT INTO data VALUES (NULL)")
	_, _ = exec.Execute("INSERT INTO data VALUES ('b')")

	// DISTINCT should treat NULLs as equal
	result, err := exec.Execute("SELECT DISTINCT value FROM data")
	if err != nil {
		t.Fatalf("SELECT DISTINCT failed: %v", err)
	}

	// Should have 3 unique values: 'a', NULL, 'b'
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 unique values (including NULL), got %d", len(result.Rows))
	}
}

func TestDistinctWithWhere(t *testing.T) {
	exec, cleanup := setupDistinctTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE items (category TEXT, name TEXT, price INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data
	_, _ = exec.Execute("INSERT INTO items VALUES ('fruit', 'apple', 10)")
	_, _ = exec.Execute("INSERT INTO items VALUES ('fruit', 'banana', 15)")
	_, _ = exec.Execute("INSERT INTO items VALUES ('fruit', 'apple', 12)") // Same name, different price
	_, _ = exec.Execute("INSERT INTO items VALUES ('vegetable', 'carrot', 8)")
	_, _ = exec.Execute("INSERT INTO items VALUES ('fruit', 'banana', 15)") // Exact duplicate

	// DISTINCT with WHERE clause
	result, err := exec.Execute("SELECT DISTINCT name FROM items WHERE category = 'fruit'")
	if err != nil {
		t.Fatalf("SELECT DISTINCT with WHERE failed: %v", err)
	}

	// Should have 2 unique fruit names: apple, banana
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 unique fruit names, got %d", len(result.Rows))
	}
}

func TestDistinctAllSame(t *testing.T) {
	exec, cleanup := setupDistinctTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE same (val TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert all same values
	_, _ = exec.Execute("INSERT INTO same VALUES ('x')")
	_, _ = exec.Execute("INSERT INTO same VALUES ('x')")
	_, _ = exec.Execute("INSERT INTO same VALUES ('x')")

	// DISTINCT should return only 1 row
	result, err := exec.Execute("SELECT DISTINCT val FROM same")
	if err != nil {
		t.Fatalf("SELECT DISTINCT failed: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestDistinctEmptyTable(t *testing.T) {
	exec, cleanup := setupDistinctTestExecutor(t)
	defer cleanup()

	// Create empty table
	_, err := exec.Execute("CREATE TABLE empty (val TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// DISTINCT on empty table
	result, err := exec.Execute("SELECT DISTINCT val FROM empty")
	if err != nil {
		t.Fatalf("SELECT DISTINCT failed: %v", err)
	}

	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
}

func TestDistinctIntegerColumn(t *testing.T) {
	exec, cleanup := setupDistinctTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE scores (score INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert duplicate scores
	_, _ = exec.Execute("INSERT INTO scores VALUES (100)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (85)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (100)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (90)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (85)")

	// DISTINCT on integer column
	result, err := exec.Execute("SELECT DISTINCT score FROM scores")
	if err != nil {
		t.Fatalf("SELECT DISTINCT failed: %v", err)
	}

	// Should have 3 unique scores: 100, 85, 90
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 unique scores, got %d", len(result.Rows))
	}
}

func TestDistinctSelectAll(t *testing.T) {
	exec, cleanup := setupDistinctTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE pairs (a INTEGER, b INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data
	_, _ = exec.Execute("INSERT INTO pairs VALUES (1, 2)")
	_, _ = exec.Execute("INSERT INTO pairs VALUES (1, 3)")
	_, _ = exec.Execute("INSERT INTO pairs VALUES (1, 2)") // Duplicate
	_, _ = exec.Execute("INSERT INTO pairs VALUES (2, 2)")

	// DISTINCT *
	result, err := exec.Execute("SELECT DISTINCT * FROM pairs")
	if err != nil {
		t.Fatalf("SELECT DISTINCT * failed: %v", err)
	}

	// Should have 3 unique rows
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 unique rows, got %d", len(result.Rows))
	}
}
