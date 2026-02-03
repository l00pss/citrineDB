package engine

import (
	"testing"
)

func TestPreparedStatement_PositionalPlaceholders(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	stmt, err := db.Prepare("INSERT INTO users (id, name, age) VALUES (?, ?, ?)")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	if stmt.ParamCount() != 3 {
		t.Errorf("Expected 3 params, got %d", stmt.ParamCount())
	}

	_, err = stmt.Execute(1, "Alice", 30)
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	_, err = stmt.Execute(2, "Bob", 25)
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	rows, err := db.Query("SELECT id, name, age FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	count := 0
	for rows.Next() {
		count++
	}
	if count != 2 {
		t.Errorf("Expected 2 rows, got %d", count)
	}
}

func TestPreparedStatement_NumberedPlaceholders(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, price REAL)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	stmt, err := db.Prepare("INSERT INTO products (id, name, price) VALUES ($1, $2, $3)")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	if stmt.ParamCount() != 3 {
		t.Errorf("Expected 3 params, got %d", stmt.ParamCount())
	}

	_, err = stmt.Execute(1, "Widget", 9.99)
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	rows, err := db.Query("SELECT name, price FROM products WHERE id = 1")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if rows.Next() {
		values := rows.Values()
		if values[0] != "Widget" {
			t.Errorf("Expected Widget, got %v", values[0])
		}
	} else {
		t.Error("Expected 1 row")
	}
}

func TestPreparedStatement_ReusedNumberedPlaceholders(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE test (a TEXT, b TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	stmt, err := db.Prepare("INSERT INTO test (a, b) VALUES ($1, $1)")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	if stmt.ParamCount() != 1 {
		t.Errorf("Expected 1 param, got %d", stmt.ParamCount())
	}

	_, err = stmt.Execute("same")
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	rows, err := db.Query("SELECT a, b FROM test")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if rows.Next() {
		values := rows.Values()
		if values[0] != "same" || values[1] != "same" {
			t.Errorf("Expected (same, same), got (%v, %v)", values[0], values[1])
		}
	}
}

func TestPreparedStatement_SQLInjectionPrevention(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE secrets (id INTEGER PRIMARY KEY, data TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Execute("INSERT INTO secrets (id, data) VALUES (1, 'secret_data')")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	stmt, err := db.Prepare("INSERT INTO secrets (id, data) VALUES (?, ?)")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	maliciousInput := "test'); DROP TABLE secrets; --"
	_, err = stmt.Execute(2, maliciousInput)
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	_, err = db.Execute("SELECT * FROM secrets")
	if err != nil {
		t.Errorf("Table was dropped! SQL injection succeeded: %v", err)
	}

	rows, err := db.Query("SELECT data FROM secrets WHERE id = 2")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if rows.Next() {
		values := rows.Values()
		if values[0] != maliciousInput {
			t.Errorf("Expected malicious input to be stored literally, got %v", values[0])
		}
	}
}

func TestPreparedStatement_NullValues(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE nullable (id INTEGER PRIMARY KEY, value TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	stmt, err := db.Prepare("INSERT INTO nullable (id, value) VALUES (?, ?)")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	_, err = stmt.Execute(1, nil)
	if err != nil {
		t.Fatalf("Failed to execute with nil: %v", err)
	}

	rows, err := db.Query("SELECT value FROM nullable WHERE id = 1")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if rows.Next() {
		values := rows.Values()
		if values[0] != nil {
			t.Errorf("Expected nil, got %v", values[0])
		}
	}
}

func TestPreparedStatement_TypeSafety(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE types (id INTEGER PRIMARY KEY, i INTEGER, f REAL, t TEXT, b BOOLEAN)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	stmt, err := db.Prepare("INSERT INTO types (id, i, f, t, b) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	_, err = stmt.Execute(1, int64(42), 3.14, "hello", true)
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}
}

func TestPreparedStatement_ParameterErrors(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	stmt, err := db.Prepare("SELECT ? + ?")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	_, err = stmt.Execute(1)
	if err == nil {
		t.Error("Expected error for too few parameters")
	}

	_, err = stmt.Execute(1, 2, 3)
	if err == nil {
		t.Error("Expected error for too many parameters")
	}
}

func TestPreparedStatement_MixedPlaceholderError(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Prepare("SELECT ? + $1")
	if err == nil {
		t.Error("Expected error for mixed placeholder styles")
	}
}

func TestPreparedStatement_SELECT(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, category TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Execute("INSERT INTO items (id, name, category) VALUES (1, 'Apple', 'Fruit')")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}
	_, err = db.Execute("INSERT INTO items (id, name, category) VALUES (2, 'Carrot', 'Vegetable')")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}
	_, err = db.Execute("INSERT INTO items (id, name, category) VALUES (3, 'Banana', 'Fruit')")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	stmt, err := db.Prepare("SELECT name FROM items WHERE category = ?")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	rows, err := stmt.Query("Fruit")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	count := 0
	for rows.Next() {
		count++
	}

	if count != 2 {
		t.Errorf("Expected 2 fruits, got %d", count)
	}
}

func TestPreparedStatement_UPDATE(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE counters (id INTEGER PRIMARY KEY, value INTEGER)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Execute("INSERT INTO counters (id, value) VALUES (1, 10)")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	stmt, err := db.Prepare("UPDATE counters SET value = ? WHERE id = ?")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	affected, err := stmt.Exec(20, 1)
	if err != nil {
		t.Fatalf("Failed to exec: %v", err)
	}

	if affected != 1 {
		t.Errorf("Expected 1 row affected, got %d", affected)
	}

	rows, err := db.Query("SELECT value FROM counters WHERE id = 1")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if rows.Next() {
		values := rows.Values()
		var val int64
		switch v := values[0].(type) {
		case int64:
			val = v
		case int32:
			val = int64(v)
		case int:
			val = int64(v)
		default:
			t.Fatalf("Unexpected type: %T", values[0])
		}
		if val != 20 {
			t.Errorf("Expected 20, got %v", val)
		}
	}
}

func TestPreparedStatement_DELETE(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Execute("INSERT INTO items (id, name) VALUES (1, 'A'), (2, 'B'), (3, 'C')")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	stmt, err := db.Prepare("DELETE FROM items WHERE id = ?")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	affected, err := stmt.Exec(2)
	if err != nil {
		t.Fatalf("Failed to exec: %v", err)
	}

	if affected != 1 {
		t.Errorf("Expected 1 row affected, got %d", affected)
	}

	rows, err := db.Query("SELECT COUNT(*) FROM items")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if rows.Next() {
		values := rows.Values()
		var val int64
		switch v := values[0].(type) {
		case int64:
			val = v
		case int32:
			val = int64(v)
		case int:
			val = int64(v)
		default:
			t.Fatalf("Unexpected type: %T", values[0])
		}
		if val != 2 {
			t.Errorf("Expected 2 items remaining, got %v", val)
		}
	}
}

func TestPreparedStatement_SpecialCharacters(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Execute("CREATE TABLE texts (id INTEGER PRIMARY KEY, content TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	stmt, err := db.Prepare("INSERT INTO texts (id, content) VALUES (?, ?)")
	if err != nil {
		t.Fatalf("Failed to prepare: %v", err)
	}

	testCases := []struct {
		id      int
		content string
	}{
		{1, "Hello 'World'"},
		{2, "Line1\nLine2"},
		{3, "Tab\there"},
		{4, "Quote \"test\""},
		{5, "Backslash \\ test"},
		// Note: Unicode handling depends on storage layer encoding
		{6, "ASCII only test"},
		{7, "Numbers 12345"},
		{8, ""},
	}

	for _, tc := range testCases {
		_, err = stmt.Execute(tc.id, tc.content)
		if err != nil {
			t.Errorf("Failed to insert content %q: %v", tc.content, err)
			continue
		}

		// Verify content was stored correctly
		queryStmt, _ := db.Prepare("SELECT content FROM texts WHERE id = ?")
		rows, err := queryStmt.Query(tc.id)
		if err != nil {
			t.Errorf("Failed to query id %d: %v", tc.id, err)
			continue
		}

		if rows.Next() {
			values := rows.Values()
			if values[0] != tc.content {
				t.Errorf("Content mismatch for id %d: expected %q, got %q", tc.id, tc.content, values[0])
			}
		}
	}
}

// Helper to create test database
func createTestDB(t *testing.T) *DB {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	return db
}
