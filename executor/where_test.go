package executor

import (
	"testing"

	"github.com/l00pss/citrinedb/storage/buffer"
	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/file"
)

func setupWhereTestExecutor(t *testing.T) (*Executor, func()) {
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

// ===== LIKE Tests =====

func TestLikeBasic(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, err := exec.Execute("CREATE TABLE products (name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, _ = exec.Execute("INSERT INTO products VALUES ('Apple')")
	_, _ = exec.Execute("INSERT INTO products VALUES ('Banana')")
	_, _ = exec.Execute("INSERT INTO products VALUES ('Apricot')")
	_, _ = exec.Execute("INSERT INTO products VALUES ('Cherry')")

	// LIKE with prefix
	result, err := exec.Execute("SELECT name FROM products WHERE name LIKE 'Ap%'")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows for 'Ap%%', got %d", len(result.Rows))
	}
}

func TestLikeSuffix(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE files (name TEXT)")
	_, _ = exec.Execute("INSERT INTO files VALUES ('document.txt')")
	_, _ = exec.Execute("INSERT INTO files VALUES ('image.png')")
	_, _ = exec.Execute("INSERT INTO files VALUES ('notes.txt')")
	_, _ = exec.Execute("INSERT INTO files VALUES ('photo.jpg')")

	// LIKE with suffix
	result, err := exec.Execute("SELECT name FROM files WHERE name LIKE '%.txt'")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 .txt files, got %d", len(result.Rows))
	}
}

func TestLikeContains(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE emails (addr TEXT)")
	_, _ = exec.Execute("INSERT INTO emails VALUES ('alice@gmail.com')")
	_, _ = exec.Execute("INSERT INTO emails VALUES ('bob@yahoo.com')")
	_, _ = exec.Execute("INSERT INTO emails VALUES ('charlie@gmail.org')")

	// LIKE with contains
	result, err := exec.Execute("SELECT addr FROM emails WHERE addr LIKE '%gmail%'")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 gmail addresses, got %d", len(result.Rows))
	}
}

func TestLikeSingleChar(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE codes (code TEXT)")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('A1')")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('A2')")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('B1')")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('AB1')")

	// LIKE with single char wildcard
	result, err := exec.Execute("SELECT code FROM codes WHERE code LIKE 'A_'")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 codes matching 'A_', got %d", len(result.Rows))
	}
}

func TestNotLike(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE items (name TEXT)")
	_, _ = exec.Execute("INSERT INTO items VALUES ('test_item')")
	_, _ = exec.Execute("INSERT INTO items VALUES ('real_item')")
	_, _ = exec.Execute("INSERT INTO items VALUES ('test_data')")

	// NOT LIKE
	result, err := exec.Execute("SELECT name FROM items WHERE name NOT LIKE 'test%'")
	if err != nil {
		t.Fatalf("NOT LIKE query failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row for NOT LIKE 'test%%', got %d", len(result.Rows))
	}
}

// ===== IN Tests =====

func TestInBasic(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE users (id INTEGER, name TEXT)")
	_, _ = exec.Execute("INSERT INTO users VALUES (1, 'Alice')")
	_, _ = exec.Execute("INSERT INTO users VALUES (2, 'Bob')")
	_, _ = exec.Execute("INSERT INTO users VALUES (3, 'Charlie')")
	_, _ = exec.Execute("INSERT INTO users VALUES (4, 'Diana')")

	// IN with integers
	result, err := exec.Execute("SELECT name FROM users WHERE id IN (1, 3)")
	if err != nil {
		t.Fatalf("IN query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows for id IN (1, 3), got %d", len(result.Rows))
	}
}

func TestInStrings(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE colors (color TEXT)")
	_, _ = exec.Execute("INSERT INTO colors VALUES ('red')")
	_, _ = exec.Execute("INSERT INTO colors VALUES ('green')")
	_, _ = exec.Execute("INSERT INTO colors VALUES ('blue')")
	_, _ = exec.Execute("INSERT INTO colors VALUES ('yellow')")

	// IN with strings
	result, err := exec.Execute("SELECT color FROM colors WHERE color IN ('red', 'blue')")
	if err != nil {
		t.Fatalf("IN query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 colors, got %d", len(result.Rows))
	}
}

func TestNotIn(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE nums (n INTEGER)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (1)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (2)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (3)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (4)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (5)")

	// NOT IN
	result, err := exec.Execute("SELECT n FROM nums WHERE n NOT IN (2, 4)")
	if err != nil {
		t.Fatalf("NOT IN query failed: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 numbers for NOT IN (2, 4), got %d", len(result.Rows))
	}
}

// ===== BETWEEN Tests =====

func TestBetweenBasic(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE scores (score INTEGER)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (50)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (75)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (85)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (95)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (100)")

	// BETWEEN inclusive
	result, err := exec.Execute("SELECT score FROM scores WHERE score BETWEEN 75 AND 95")
	if err != nil {
		t.Fatalf("BETWEEN query failed: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 scores between 75 and 95, got %d", len(result.Rows))
	}
}

func TestNotBetween(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE ages (age INTEGER)")
	_, _ = exec.Execute("INSERT INTO ages VALUES (15)")
	_, _ = exec.Execute("INSERT INTO ages VALUES (25)")
	_, _ = exec.Execute("INSERT INTO ages VALUES (35)")
	_, _ = exec.Execute("INSERT INTO ages VALUES (45)")

	// NOT BETWEEN
	result, err := exec.Execute("SELECT age FROM ages WHERE age NOT BETWEEN 20 AND 40")
	if err != nil {
		t.Fatalf("NOT BETWEEN query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 ages not between 20 and 40, got %d", len(result.Rows))
	}
}

func TestBetweenBoundary(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE vals (v INTEGER)")
	_, _ = exec.Execute("INSERT INTO vals VALUES (10)")
	_, _ = exec.Execute("INSERT INTO vals VALUES (20)")
	_, _ = exec.Execute("INSERT INTO vals VALUES (30)")

	// BETWEEN should be inclusive on both ends
	result, err := exec.Execute("SELECT v FROM vals WHERE v BETWEEN 10 AND 30")
	if err != nil {
		t.Fatalf("BETWEEN query failed: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 values (inclusive), got %d", len(result.Rows))
	}
}

// ===== RIGHT JOIN Tests =====

func TestRightJoinWithNulls(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE orders (id INTEGER, customer_id INTEGER)")
	_, _ = exec.Execute("CREATE TABLE customers (id INTEGER, name TEXT)")

	_, _ = exec.Execute("INSERT INTO customers VALUES (1, 'Alice')")
	_, _ = exec.Execute("INSERT INTO customers VALUES (2, 'Bob')")
	_, _ = exec.Execute("INSERT INTO customers VALUES (3, 'Charlie')")

	_, _ = exec.Execute("INSERT INTO orders VALUES (100, 1)")
	_, _ = exec.Execute("INSERT INTO orders VALUES (101, 1)")
	// Customer 2 and 3 have no orders

	// RIGHT JOIN should include all customers
	result, err := exec.Execute("SELECT c.name, o.id FROM orders o RIGHT JOIN customers c ON o.customer_id = c.id")
	if err != nil {
		t.Fatalf("RIGHT JOIN query failed: %v", err)
	}
	// Should have: Alice-100, Alice-101, Bob-NULL, Charlie-NULL
	if len(result.Rows) != 4 {
		t.Errorf("expected 4 rows from RIGHT JOIN, got %d", len(result.Rows))
	}
}

// ===== FULL OUTER JOIN Tests =====

func TestFullOuterJoin(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE left_t (id INTEGER, val TEXT)")
	_, _ = exec.Execute("CREATE TABLE right_t (id INTEGER, data TEXT)")

	_, _ = exec.Execute("INSERT INTO left_t VALUES (1, 'a')")
	_, _ = exec.Execute("INSERT INTO left_t VALUES (2, 'b')")
	_, _ = exec.Execute("INSERT INTO left_t VALUES (3, 'c')")

	_, _ = exec.Execute("INSERT INTO right_t VALUES (2, 'x')")
	_, _ = exec.Execute("INSERT INTO right_t VALUES (3, 'y')")
	_, _ = exec.Execute("INSERT INTO right_t VALUES (4, 'z')")

	// FULL OUTER JOIN should include all from both tables
	result, err := exec.Execute("SELECT l.val, r.data FROM left_t l FULL JOIN right_t r ON l.id = r.id")
	if err != nil {
		t.Fatalf("FULL JOIN query failed: %v", err)
	}
	// Should have: a-NULL, b-x, c-y, NULL-z = 4 rows
	if len(result.Rows) != 4 {
		t.Errorf("expected 4 rows from FULL JOIN, got %d", len(result.Rows))
	}
}

// ===== GLOB Tests =====

func TestGlobBasic(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE files (name TEXT)")
	_, _ = exec.Execute("INSERT INTO files VALUES ('test.txt')")
	_, _ = exec.Execute("INSERT INTO files VALUES ('Test.txt')")
	_, _ = exec.Execute("INSERT INTO files VALUES ('TEST.TXT')")

	// GLOB is case-sensitive, * matches any
	result, err := exec.Execute("SELECT name FROM files WHERE name GLOB 'test*'")
	if err != nil {
		t.Fatalf("GLOB query failed: %v", err)
	}
	// Only 'test.txt' should match (case-sensitive)
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row for case-sensitive GLOB, got %d", len(result.Rows))
	}
}

func TestGlobSingleChar(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE codes (code TEXT)")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('A1B')")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('A2B')")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('A12B')")

	// GLOB with ? for single char
	result, err := exec.Execute("SELECT code FROM codes WHERE code GLOB 'A?B'")
	if err != nil {
		t.Fatalf("GLOB query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 codes matching 'A?B', got %d", len(result.Rows))
	}
}

// ===== Combined Tests =====

func TestCombinedConditions(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE products (id INTEGER, category TEXT, price INTEGER)")
	_, _ = exec.Execute("INSERT INTO products VALUES (1, 'Electronics', 100)")
	_, _ = exec.Execute("INSERT INTO products VALUES (2, 'Electronics', 200)")
	_, _ = exec.Execute("INSERT INTO products VALUES (3, 'Clothing', 50)")
	_, _ = exec.Execute("INSERT INTO products VALUES (4, 'Clothing', 75)")
	_, _ = exec.Execute("INSERT INTO products VALUES (5, 'Food', 25)")

	// Combined: IN and BETWEEN
	result, err := exec.Execute("SELECT id FROM products WHERE category IN ('Electronics', 'Clothing') AND price BETWEEN 50 AND 150")
	if err != nil {
		t.Fatalf("Combined query failed: %v", err)
	}

	// Electronics 100 (in range), Electronics 200 (out of range), Clothing 50, Clothing 75 = 3
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 products (100, 50, 75 in range), got %d", len(result.Rows))
	}
}
