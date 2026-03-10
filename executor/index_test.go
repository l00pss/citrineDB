package executor

import (
	"strings"
	"testing"

	"github.com/l00pss/citrinedb/storage/buffer"
	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/file"
)

func setupIndexTestExecutor(t *testing.T) (*Executor, func()) {
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

func TestCreateIndex(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table first
	_, err := exec.Execute("CREATE TABLE users (id INTEGER, name TEXT, age INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert some data
	_, err = exec.Execute("INSERT INTO users VALUES (1, 'Alice', 30)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO users VALUES (2, 'Bob', 25)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO users VALUES (3, 'Charlie', 35)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create index
	result, err := exec.Execute("CREATE INDEX idx_users_age ON users (age)")
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	if !strings.Contains(result.Message, "Index 'idx_users_age' created") {
		t.Errorf("unexpected message: %s", result.Message)
	}
	if !strings.Contains(result.Message, "3 entries") {
		t.Errorf("expected 3 entries in index, got: %s", result.Message)
	}
}

func TestCreateUniqueIndex(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE products (id INTEGER, sku TEXT, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data with unique SKUs
	_, err = exec.Execute("INSERT INTO products VALUES (1, 'SKU001', 'Product A')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO products VALUES (2, 'SKU002', 'Product B')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create unique index
	result, err := exec.Execute("CREATE UNIQUE INDEX idx_products_sku ON products (sku)")
	if err != nil {
		t.Fatalf("failed to create unique index: %v", err)
	}

	if !strings.Contains(result.Message, "Unique index") {
		t.Errorf("expected unique index message, got: %s", result.Message)
	}
}

func TestCreateUniqueIndexViolation(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE items (id INTEGER, code TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert duplicate values
	_, err = exec.Execute("INSERT INTO items VALUES (1, 'ABC')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO items VALUES (2, 'ABC')") // Duplicate!
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Try to create unique index - should fail
	_, err = exec.Execute("CREATE UNIQUE INDEX idx_items_code ON items (code)")
	if err == nil {
		t.Fatal("expected unique constraint violation error")
	}
	if !strings.Contains(err.Error(), "unique constraint violated") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateIndexIfNotExists(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE test (id INTEGER, value TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Create index
	_, err = exec.Execute("CREATE INDEX idx_test ON test (id)")
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Create same index with IF NOT EXISTS - should not error
	result, err := exec.Execute("CREATE INDEX IF NOT EXISTS idx_test ON test (id)")
	if err != nil {
		t.Fatalf("expected no error with IF NOT EXISTS, got: %v", err)
	}
	if !strings.Contains(result.Message, "already exists") {
		t.Errorf("expected 'already exists' message, got: %s", result.Message)
	}
}

func TestDropIndex(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table and index
	_, err := exec.Execute("CREATE TABLE test (id INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = exec.Execute("CREATE INDEX idx_test_id ON test (id)")
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Drop index
	result, err := exec.Execute("DROP INDEX idx_test_id")
	if err != nil {
		t.Fatalf("failed to drop index: %v", err)
	}
	if !strings.Contains(result.Message, "dropped successfully") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestDropIndexIfExists(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Drop non-existent index with IF EXISTS - should not error
	result, err := exec.Execute("DROP INDEX IF EXISTS non_existent_idx")
	if err != nil {
		t.Fatalf("expected no error with IF EXISTS, got: %v", err)
	}
	if !strings.Contains(result.Message, "does not exist") {
		t.Errorf("expected 'does not exist' message, got: %s", result.Message)
	}
}

func TestDropIndexNotFound(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Try to drop non-existent index without IF EXISTS
	_, err := exec.Execute("DROP INDEX non_existent")
	if err == nil {
		t.Fatal("expected error when dropping non-existent index")
	}
}

func TestIndexMaintainedOnInsert(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table and index
	_, err := exec.Execute("CREATE TABLE users (id INTEGER, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = exec.Execute("CREATE INDEX idx_users_id ON users (id)")
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Insert data after index creation
	_, err = exec.Execute("INSERT INTO users VALUES (1, 'Alice')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO users VALUES (2, 'Bob')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Query to verify data exists
	result, err := exec.Execute("SELECT * FROM users")
	if err != nil {
		t.Fatalf("failed to select: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}
}

func TestIndexUpdatedOnUpdate(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table with data
	_, err := exec.Execute("CREATE TABLE users (id INTEGER, name TEXT, score INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = exec.Execute("INSERT INTO users VALUES (1, 'Alice', 100)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO users VALUES (2, 'Bob', 200)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create index on score column
	_, err = exec.Execute("CREATE INDEX idx_users_score ON users (score)")
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Update a record's score - this should update the index
	result, err := exec.Execute("UPDATE users SET score = 150 WHERE id = 1")
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}
	if result.RowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", result.RowsAffected)
	}

	// Verify the update
	result, err = exec.Execute("SELECT score FROM users WHERE id = 1")
	if err != nil {
		t.Fatalf("failed to select: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	// Score should be 150 now
	score := result.Rows[0][0]
	if score != int32(150) && score != 150 {
		t.Errorf("expected score 150, got %v", score)
	}
}

func TestIndexUpdatedOnDelete(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table with data
	_, err := exec.Execute("CREATE TABLE products (id INTEGER, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = exec.Execute("INSERT INTO products VALUES (1, 'Product A')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO products VALUES (2, 'Product B')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO products VALUES (3, 'Product C')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create index
	_, err = exec.Execute("CREATE INDEX idx_products_id ON products (id)")
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Delete a record
	result, err := exec.Execute("DELETE FROM products WHERE id = 2")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}
	if result.RowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", result.RowsAffected)
	}

	// Verify deletion
	result, err = exec.Execute("SELECT * FROM products")
	if err != nil {
		t.Fatalf("failed to select: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows after delete, got %d", len(result.Rows))
	}
}

func TestCompositeIndex(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE orders (customer_id INTEGER, order_date TEXT, amount INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data
	_, err = exec.Execute("INSERT INTO orders VALUES (1, '2024-01-01', 100)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO orders VALUES (1, '2024-01-02', 200)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO orders VALUES (2, '2024-01-01', 150)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create composite index on (customer_id, order_date)
	result, err := exec.Execute("CREATE INDEX idx_orders_customer_date ON orders (customer_id, order_date)")
	if err != nil {
		t.Fatalf("failed to create composite index: %v", err)
	}
	if !strings.Contains(result.Message, "customer_id, order_date") {
		t.Errorf("expected composite columns in message, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "3 entries") {
		t.Errorf("expected 3 entries, got: %s", result.Message)
	}
}

func TestIndexOnNonExistentColumn(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE test (id INTEGER, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Try to create index on non-existent column
	_, err = exec.Execute("CREATE INDEX idx_test ON test (nonexistent)")
	if err == nil {
		t.Fatal("expected error for non-existent column")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIndexOnNonExistentTable(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Try to create index on non-existent table
	_, err := exec.Execute("CREATE INDEX idx_test ON nonexistent_table (id)")
	if err == nil {
		t.Fatal("expected error for non-existent table")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUniqueIndexEnforcedOnInsert(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table with unique index
	_, err := exec.Execute("CREATE TABLE users (id INTEGER, email TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = exec.Execute("CREATE UNIQUE INDEX idx_users_email ON users (email)")
	if err != nil {
		t.Fatalf("failed to create unique index: %v", err)
	}

	// Insert first record
	_, err = exec.Execute("INSERT INTO users VALUES (1, 'alice@example.com')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Try to insert duplicate email - should fail
	_, err = exec.Execute("INSERT INTO users VALUES (2, 'alice@example.com')")
	if err == nil {
		t.Fatal("expected unique constraint violation on insert")
	}
}

func TestMultipleIndexesOnTable(t *testing.T) {
	exec, cleanup := setupIndexTestExecutor(t)
	defer cleanup()

	// Create table
	_, err := exec.Execute("CREATE TABLE employees (id INTEGER, name TEXT, dept TEXT, salary INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data
	_, err = exec.Execute("INSERT INTO employees VALUES (1, 'Alice', 'Engineering', 100000)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = exec.Execute("INSERT INTO employees VALUES (2, 'Bob', 'Sales', 80000)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create multiple indexes
	_, err = exec.Execute("CREATE INDEX idx_emp_id ON employees (id)")
	if err != nil {
		t.Fatalf("failed to create index 1: %v", err)
	}
	_, err = exec.Execute("CREATE INDEX idx_emp_dept ON employees (dept)")
	if err != nil {
		t.Fatalf("failed to create index 2: %v", err)
	}
	_, err = exec.Execute("CREATE INDEX idx_emp_salary ON employees (salary)")
	if err != nil {
		t.Fatalf("failed to create index 3: %v", err)
	}

	// Insert more data - should update all indexes
	_, err = exec.Execute("INSERT INTO employees VALUES (3, 'Charlie', 'Engineering', 90000)")
	if err != nil {
		t.Fatalf("failed to insert after index creation: %v", err)
	}

	// Verify data
	result, err := exec.Execute("SELECT * FROM employees")
	if err != nil {
		t.Fatalf("failed to select: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result.Rows))
	}
}
