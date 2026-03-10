package executor

import (
	"fmt"
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

// ===== LIKE Edge Cases =====

func TestLikeCaseInsensitive(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE names (name TEXT)")
	_, _ = exec.Execute("INSERT INTO names VALUES ('HELLO')")
	_, _ = exec.Execute("INSERT INTO names VALUES ('Hello')")
	_, _ = exec.Execute("INSERT INTO names VALUES ('hello')")
	_, _ = exec.Execute("INSERT INTO names VALUES ('HeLLo')")

	// LIKE should be case-insensitive
	result, err := exec.Execute("SELECT name FROM names WHERE name LIKE 'hello'")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 4 {
		t.Errorf("expected 4 rows (case-insensitive), got %d", len(result.Rows))
	}
}

func TestLikeEmptyPattern(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE data (val TEXT)")
	_, _ = exec.Execute("INSERT INTO data VALUES ('abc')")
	_, _ = exec.Execute("INSERT INTO data VALUES ('')")

	// Empty pattern should match empty string only
	result, err := exec.Execute("SELECT val FROM data WHERE val LIKE ''")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 empty string match, got %d", len(result.Rows))
	}
}

func TestLikePercentOnly(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE data (val TEXT)")
	_, _ = exec.Execute("INSERT INTO data VALUES ('abc')")
	_, _ = exec.Execute("INSERT INTO data VALUES ('')")
	_, _ = exec.Execute("INSERT INTO data VALUES ('xyz')")

	// Single % should match everything
	result, err := exec.Execute("SELECT val FROM data WHERE val LIKE '%'")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows for '%%', got %d", len(result.Rows))
	}
}

func TestLikeMultipleWildcards(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE paths (path TEXT)")
	_, _ = exec.Execute("INSERT INTO paths VALUES ('/home/user/doc.txt')")
	_, _ = exec.Execute("INSERT INTO paths VALUES ('/var/log/sys.log')")
	_, _ = exec.Execute("INSERT INTO paths VALUES ('/home/admin/notes.txt')")
	_, _ = exec.Execute("INSERT INTO paths VALUES ('/tmp/file.tmp')")

	// Multiple % wildcards
	result, err := exec.Execute("SELECT path FROM paths WHERE path LIKE '/home/%/%.txt'")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 paths matching '/home/%%/%%.txt', got %d", len(result.Rows))
	}
}

func TestLikeUnderscoreMultiple(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE codes (code TEXT)")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('AB')")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('ABC')")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('ABCD')")
	_, _ = exec.Execute("INSERT INTO codes VALUES ('A')")

	// Multiple underscores for exact length
	result, err := exec.Execute("SELECT code FROM codes WHERE code LIKE '___'")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 code with exactly 3 chars, got %d", len(result.Rows))
	}
}

func TestLikeMixedWildcards(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE items (sku TEXT)")
	_, _ = exec.Execute("INSERT INTO items VALUES ('A-001-XL')")
	_, _ = exec.Execute("INSERT INTO items VALUES ('A-002-XL')")
	_, _ = exec.Execute("INSERT INTO items VALUES ('B-001-XL')")
	_, _ = exec.Execute("INSERT INTO items VALUES ('A-001-SM')")

	// Mix of % and _
	result, err := exec.Execute("SELECT sku FROM items WHERE sku LIKE 'A-00_-XL'")
	if err != nil {
		t.Fatalf("LIKE query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 items matching 'A-00_-XL', got %d", len(result.Rows))
	}
}

// ===== GLOB Edge Cases =====

func TestGlobNotGlob(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE files (name TEXT)")
	_, _ = exec.Execute("INSERT INTO files VALUES ('test.go')")
	_, _ = exec.Execute("INSERT INTO files VALUES ('main.go')")
	_, _ = exec.Execute("INSERT INTO files VALUES ('test.txt')")

	// NOT GLOB
	result, err := exec.Execute("SELECT name FROM files WHERE name NOT GLOB 'test*'")
	if err != nil {
		t.Fatalf("NOT GLOB query failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 file not matching 'test*', got %d", len(result.Rows))
	}
}

func TestGlobVsLikeCaseSensitivity(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE words (word TEXT)")
	_, _ = exec.Execute("INSERT INTO words VALUES ('Apple')")
	_, _ = exec.Execute("INSERT INTO words VALUES ('apple')")
	_, _ = exec.Execute("INSERT INTO words VALUES ('APPLE')")

	// LIKE is case-insensitive
	likeResult, _ := exec.Execute("SELECT word FROM words WHERE word LIKE 'apple'")
	// GLOB is case-sensitive
	globResult, _ := exec.Execute("SELECT word FROM words WHERE word GLOB 'apple'")

	if len(likeResult.Rows) != 3 {
		t.Errorf("LIKE should match 3 (case-insensitive), got %d", len(likeResult.Rows))
	}
	if len(globResult.Rows) != 1 {
		t.Errorf("GLOB should match 1 (case-sensitive), got %d", len(globResult.Rows))
	}
}

// ===== IN Edge Cases =====

func TestInSingleValue(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE nums (n INTEGER)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (1)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (2)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (3)")

	// IN with single value (same as =)
	result, err := exec.Execute("SELECT n FROM nums WHERE n IN (2)")
	if err != nil {
		t.Fatalf("IN query failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row for single value IN, got %d", len(result.Rows))
	}
}

func TestInManyValues(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE data (id INTEGER)")
	for i := 1; i <= 10; i++ {
		exec.Execute(fmt.Sprintf("INSERT INTO data VALUES (%d)", i))
	}

	// IN with many values
	result, err := exec.Execute("SELECT id FROM data WHERE id IN (1, 3, 5, 7, 9)")
	if err != nil {
		t.Fatalf("IN query failed: %v", err)
	}
	if len(result.Rows) != 5 {
		t.Errorf("expected 5 odd numbers, got %d", len(result.Rows))
	}
}

func TestInNoMatch(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE data (val TEXT)")
	_, _ = exec.Execute("INSERT INTO data VALUES ('a')")
	_, _ = exec.Execute("INSERT INTO data VALUES ('b')")

	// IN with no matching values
	result, err := exec.Execute("SELECT val FROM data WHERE val IN ('x', 'y', 'z')")
	if err != nil {
		t.Fatalf("IN query failed: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows for no match, got %d", len(result.Rows))
	}
}

func TestInWithOr(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE products (id INTEGER, category TEXT)")
	_, _ = exec.Execute("INSERT INTO products VALUES (1, 'A')")
	_, _ = exec.Execute("INSERT INTO products VALUES (2, 'B')")
	_, _ = exec.Execute("INSERT INTO products VALUES (3, 'C')")
	_, _ = exec.Execute("INSERT INTO products VALUES (4, 'A')")

	// IN combined with OR
	result, err := exec.Execute("SELECT id FROM products WHERE id IN (1, 2) OR category = 'C'")
	if err != nil {
		t.Fatalf("IN+OR query failed: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows (1, 2, 3), got %d", len(result.Rows))
	}
}

// ===== BETWEEN Edge Cases =====

func TestBetweenLargeRange(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE temps (temp INTEGER)")
	_, _ = exec.Execute("INSERT INTO temps VALUES (0)")
	_, _ = exec.Execute("INSERT INTO temps VALUES (10)")
	_, _ = exec.Execute("INSERT INTO temps VALUES (50)")
	_, _ = exec.Execute("INSERT INTO temps VALUES (100)")
	_, _ = exec.Execute("INSERT INTO temps VALUES (200)")

	// BETWEEN with large range
	result, err := exec.Execute("SELECT temp FROM temps WHERE temp BETWEEN 5 AND 150")
	if err != nil {
		t.Fatalf("BETWEEN query failed: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 temps (10, 50, 100), got %d", len(result.Rows))
	}
}

func TestBetweenSameValue(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE nums (n INTEGER)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (5)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (10)")
	_, _ = exec.Execute("INSERT INTO nums VALUES (15)")

	// BETWEEN with same low and high (should match exactly that value)
	result, err := exec.Execute("SELECT n FROM nums WHERE n BETWEEN 10 AND 10")
	if err != nil {
		t.Fatalf("BETWEEN query failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row for same boundary, got %d", len(result.Rows))
	}
}

func TestBetweenWithOr(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE scores (score INTEGER)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (10)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (50)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (90)")
	_, _ = exec.Execute("INSERT INTO scores VALUES (95)")

	// Multiple BETWEEN with OR
	result, err := exec.Execute("SELECT score FROM scores WHERE score BETWEEN 0 AND 20 OR score BETWEEN 80 AND 100")
	if err != nil {
		t.Fatalf("BETWEEN+OR query failed: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 scores (10, 90, 95), got %d", len(result.Rows))
	}
}

// ===== JOIN Edge Cases =====

func TestRightJoinAllMatch(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE a (id INTEGER)")
	_, _ = exec.Execute("CREATE TABLE b (id INTEGER)")
	_, _ = exec.Execute("INSERT INTO a VALUES (1)")
	_, _ = exec.Execute("INSERT INTO a VALUES (2)")
	_, _ = exec.Execute("INSERT INTO b VALUES (1)")
	_, _ = exec.Execute("INSERT INTO b VALUES (2)")

	// All rows match
	result, err := exec.Execute("SELECT a.id, b.id FROM a RIGHT JOIN b ON a.id = b.id")
	if err != nil {
		t.Fatalf("RIGHT JOIN query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows (all match), got %d", len(result.Rows))
	}
}

func TestRightJoinNoMatch(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE a (id INTEGER)")
	_, _ = exec.Execute("CREATE TABLE b (id INTEGER)")
	_, _ = exec.Execute("INSERT INTO a VALUES (1)")
	_, _ = exec.Execute("INSERT INTO a VALUES (2)")
	_, _ = exec.Execute("INSERT INTO b VALUES (3)")
	_, _ = exec.Execute("INSERT INTO b VALUES (4)")

	// No matches - all right rows with NULL left
	result, err := exec.Execute("SELECT a.id, b.id FROM a RIGHT JOIN b ON a.id = b.id")
	if err != nil {
		t.Fatalf("RIGHT JOIN query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows (no match, all NULL-left), got %d", len(result.Rows))
	}
}

func TestFullJoinEmpty(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE a (id INTEGER)")
	_, _ = exec.Execute("CREATE TABLE b (id INTEGER)")
	_, _ = exec.Execute("INSERT INTO a VALUES (1)")
	// b is empty

	// FULL JOIN with empty right table
	result, err := exec.Execute("SELECT a.id, b.id FROM a FULL JOIN b ON a.id = b.id")
	if err != nil {
		t.Fatalf("FULL JOIN query failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row (left only), got %d", len(result.Rows))
	}
}

func TestFullJoinAllMatch(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE x (id INTEGER, val TEXT)")
	_, _ = exec.Execute("CREATE TABLE y (id INTEGER, data TEXT)")
	_, _ = exec.Execute("INSERT INTO x VALUES (1, 'a')")
	_, _ = exec.Execute("INSERT INTO x VALUES (2, 'b')")
	_, _ = exec.Execute("INSERT INTO y VALUES (1, 'x')")
	_, _ = exec.Execute("INSERT INTO y VALUES (2, 'y')")

	// All rows match
	result, err := exec.Execute("SELECT x.val, y.data FROM x FULL JOIN y ON x.id = y.id")
	if err != nil {
		t.Fatalf("FULL JOIN query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows (all match), got %d", len(result.Rows))
	}
}

// ===== Complex Combined Tests =====

func TestLikeAndBetween(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE logs (level TEXT, code INTEGER)")
	_, _ = exec.Execute("INSERT INTO logs VALUES ('ERROR', 500)")
	_, _ = exec.Execute("INSERT INTO logs VALUES ('ERROR', 404)")
	_, _ = exec.Execute("INSERT INTO logs VALUES ('WARN', 300)")
	_, _ = exec.Execute("INSERT INTO logs VALUES ('INFO', 200)")

	// LIKE and BETWEEN combined
	result, err := exec.Execute("SELECT level, code FROM logs WHERE level LIKE 'E%' AND code BETWEEN 400 AND 600")
	if err != nil {
		t.Fatalf("Combined query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 ERROR logs with code 400-600, got %d", len(result.Rows))
	}
}

func TestNotInAndNotBetween(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE items (category TEXT, price INTEGER)")
	_, _ = exec.Execute("INSERT INTO items VALUES ('A', 10)")
	_, _ = exec.Execute("INSERT INTO items VALUES ('B', 50)")
	_, _ = exec.Execute("INSERT INTO items VALUES ('C', 100)")
	_, _ = exec.Execute("INSERT INTO items VALUES ('A', 200)")

	// NOT IN and NOT BETWEEN
	result, err := exec.Execute("SELECT category, price FROM items WHERE category NOT IN ('B', 'C') AND price NOT BETWEEN 100 AND 300")
	if err != nil {
		t.Fatalf("Combined query failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 item (A-10), got %d", len(result.Rows))
	}
}

func TestTripleConditions(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE orders (id INTEGER, status TEXT, amount INTEGER)")
	_, _ = exec.Execute("INSERT INTO orders VALUES (1, 'pending', 100)")
	_, _ = exec.Execute("INSERT INTO orders VALUES (2, 'completed', 200)")
	_, _ = exec.Execute("INSERT INTO orders VALUES (3, 'pending', 300)")
	_, _ = exec.Execute("INSERT INTO orders VALUES (4, 'cancelled', 150)")
	_, _ = exec.Execute("INSERT INTO orders VALUES (5, 'completed', 50)")

	// Three conditions: IN, BETWEEN, and comparison
	result, err := exec.Execute("SELECT id FROM orders WHERE status IN ('pending', 'completed') AND amount BETWEEN 50 AND 250 AND id > 1")
	if err != nil {
		t.Fatalf("Triple condition query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 orders (2, 5), got %d", len(result.Rows))
	}
}

func TestJoinWithWhere(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE employees (id INTEGER, name TEXT, dept_id INTEGER)")
	_, _ = exec.Execute("CREATE TABLE departments (id INTEGER, name TEXT)")

	_, _ = exec.Execute("INSERT INTO departments VALUES (1, 'Engineering')")
	_, _ = exec.Execute("INSERT INTO departments VALUES (2, 'Sales')")
	_, _ = exec.Execute("INSERT INTO departments VALUES (3, 'HR')")

	_, _ = exec.Execute("INSERT INTO employees VALUES (1, 'Alice', 1)")
	_, _ = exec.Execute("INSERT INTO employees VALUES (2, 'Bob', 1)")
	_, _ = exec.Execute("INSERT INTO employees VALUES (3, 'Charlie', 2)")

	// INNER JOIN with WHERE using LIKE
	result, err := exec.Execute("SELECT e.name, d.name FROM employees e JOIN departments d ON e.dept_id = d.id WHERE d.name LIKE 'Eng%'")
	if err != nil {
		t.Fatalf("JOIN with WHERE query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 Engineering employees, got %d", len(result.Rows))
	}
}

func TestInnerJoinWithIn(t *testing.T) {
	exec, cleanup := setupWhereTestExecutor(t)
	defer cleanup()

	_, _ = exec.Execute("CREATE TABLE projects (id INTEGER, name TEXT)")
	_, _ = exec.Execute("CREATE TABLE tasks (id INTEGER, project_id INTEGER, status TEXT)")

	_, _ = exec.Execute("INSERT INTO projects VALUES (1, 'Alpha')")
	_, _ = exec.Execute("INSERT INTO projects VALUES (2, 'Beta')")
	_, _ = exec.Execute("INSERT INTO projects VALUES (3, 'Gamma')")

	_, _ = exec.Execute("INSERT INTO tasks VALUES (1, 1, 'done')")
	_, _ = exec.Execute("INSERT INTO tasks VALUES (2, 1, 'pending')")
	_, _ = exec.Execute("INSERT INTO tasks VALUES (3, 2, 'done')")
	_, _ = exec.Execute("INSERT INTO tasks VALUES (4, 3, 'pending')")

	// INNER JOIN with IN filter
	result, err := exec.Execute("SELECT p.name, t.status FROM projects p JOIN tasks t ON p.id = t.project_id WHERE t.status IN ('done')")
	if err != nil {
		t.Fatalf("JOIN with IN query failed: %v", err)
	}
	// Alpha-done, Beta-done = 2
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 done tasks, got %d", len(result.Rows))
	}
}
