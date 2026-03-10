# CitrineDB User Guide

*Version: 0.1.0*  
*Last Updated: February 4, 2026*

---

## Table of Contents

1. [Introduction](#introduction)
2. [Installation](#installation)
3. [Quick Start](#quick-start)
4. [CLI Usage](#cli-usage)
5. [Programmatic API](#programmatic-api)
   - [Prepared Statements](#prepared-statements)
6. [SQL Reference](#sql-reference)
7. [Data Types](#data-types)
8. [Known Limitations](#known-limitations)
9. [Examples](#examples)

---

## Introduction

CitrineDB is a lightweight, embedded SQL database written in Go. It features:

- **Embedded Architecture** - No separate server process required
- **SQL Support** - Standard SQL syntax for queries
- **ACID Transactions** - Write-Ahead Logging (WAL) for durability
- **B-Tree Indexes** - Fast key-value lookups
- **Buffer Pool** - LRU-based page caching
- **Slotted Pages** - Efficient variable-length record storage

---

## Installation

### As a Library

```bash
go get github.com/l00pss/citrinedb
```

### Build CLI

```bash
git clone https://github.com/l00pss/citrinedb.git
cd citrinedb
go build -o citrinedb ./cmd/citrinedb
```

---

## Quick Start

### CLI Mode

```bash
# In-memory database
./citrinedb

# File-based database
./citrinedb mydata.db
```

### Programmatic Mode

```go
package main

import (
    "fmt"
    "log"
    "github.com/l00pss/citrinedb/engine"
)

func main() {
    db, err := engine.Open("mydata.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create table
    db.Execute("CREATE TABLE users (id INTEGER, name TEXT)")
    
    // Insert data
    db.Execute("INSERT INTO users VALUES (1, 'Alice')")
    
    // Query data
    rows, _ := db.Query("SELECT * FROM users")
    for rows.Next() {
        var id int
        var name string
        rows.Scan(&id, &name)
        fmt.Printf("ID: %d, Name: %s\n", id, name)
    }
}
```

---

## CLI Usage

### Starting the CLI

```bash
# Show version
./citrinedb --version

# In-memory database (data lost on exit)
./citrinedb

# Persistent database
./citrinedb /path/to/database.db
```

### Dot Commands

| Command | Description |
|---------|-------------|
| `.help` | Show available commands |
| `.quit` | Exit the CLI |
| `.tables` | List all tables |
| `.schema TABLE` | Show table schema |
| `.stats` | Show database statistics |
| `.read FILE` | Execute SQL from file |

### Interactive SQL

```sql
citrinedb> CREATE TABLE products (
      ...>   id INTEGER NOT NULL,
      ...>   name TEXT,
      ...>   price REAL
      ...> );
Table 'products' created successfully

citrinedb> INSERT INTO products VALUES (1, 'Widget', 29.99);
1 row(s) inserted

citrinedb> SELECT * FROM products;
id | name   | price
---+--------+------
1  | Widget | 29.99
(1 rows)
```

### Multi-line Statements

SQL statements can span multiple lines. Press Enter to continue, and end with `;`:

```sql
citrinedb> SELECT id, name, price
      ...> FROM products
      ...> WHERE price > 20
      ...> ORDER BY name;
```

### Executing SQL Files

```bash
# From command line
echo ".read schema.sql" | ./citrinedb mydata.db

# From interactive mode
citrinedb> .read data/import.sql
Executed: 1000 statements, Errors: 0
```

---

## Programmatic API

### Opening a Database

```go
import "github.com/l00pss/citrinedb/engine"

// Simple open (default settings)
db, err := engine.Open("mydata.db")

// Open with configuration
config := engine.Config{
    Path:           "mydata.db",
    PageSize:       4096,       // Page size in bytes
    BufferPoolSize: 1024,       // Number of pages in buffer pool
    WALDir:         "mydata.db.wal",
    SyncWrites:     true,       // Sync after each write
}
db, err := engine.OpenWithConfig(config)
```

### Closing the Database

```go
defer db.Close()
```

### Execute (DDL/DML)

For statements that don't return rows (CREATE, INSERT, UPDATE, DELETE):

```go
// Using Execute - returns full result
result, err := db.Execute("CREATE TABLE users (id INTEGER, name TEXT)")
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.Message)

// Using Exec - returns rows affected
rowsAffected, err := db.Exec("INSERT INTO users VALUES (1, 'Alice')")
fmt.Printf("Inserted %d rows\n", rowsAffected)
```

### Query (SELECT)

For queries that return rows:

```go
rows, err := db.Query("SELECT id, name FROM users WHERE id = 1")
if err != nil {
    log.Fatal(err)
}

// Get column names
columns := rows.Columns()
fmt.Println("Columns:", columns)

// Iterate through rows
for rows.Next() {
    var id int
    var name string
    err := rows.Scan(&id, &name)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("ID: %d, Name: %s\n", id, name)
}

// Or get row count
fmt.Printf("Total rows: %d\n", rows.Count())
```

### Transactions

```go
// Begin transaction
db.Execute("BEGIN TRANSACTION")

// Perform operations
db.Execute("UPDATE accounts SET balance = balance - 100 WHERE id = 1")
db.Execute("UPDATE accounts SET balance = balance + 100 WHERE id = 2")

// Commit
db.Execute("COMMIT")

// Or rollback on error
db.Execute("ROLLBACK")
```

### Prepared Statements

Prepared statements prevent SQL injection attacks and improve performance for repeated queries.

#### Creating a Prepared Statement

```go
// Positional placeholders (?)
stmt, err := db.Prepare("INSERT INTO users (id, name, age) VALUES (?, ?, ?)")
if err != nil {
    log.Fatal(err)
}
defer stmt.Close()

// Numbered placeholders ($1, $2, ...)
stmt, err := db.Prepare("SELECT * FROM users WHERE id = $1 AND status = $2")
```

#### Executing Prepared Statements

```go
// Execute - returns full result (for any statement type)
result, err := stmt.Execute(1, "Alice", 30)

// Query - returns rows iterator (for SELECT)
rows, err := stmt.Query("Fruit")
for rows.Next() {
    values := rows.Values()
    // process values...
}

// Exec - returns rows affected (for INSERT/UPDATE/DELETE)
affected, err := stmt.Exec(newValue, id)
```

#### Reusing Prepared Statements

```go
stmt, _ := db.Prepare("INSERT INTO logs (level, message) VALUES (?, ?)")

// Execute multiple times with different parameters
stmt.Exec("INFO", "Application started")
stmt.Exec("DEBUG", "Loading configuration")
stmt.Exec("WARN", "Cache miss for key: user_123")
stmt.Exec("ERROR", "Database connection timeout")
```

#### Reusing Parameters (Numbered Style)

```go
// $1 is used twice - only 1 parameter needed
stmt, _ := db.Prepare("INSERT INTO pairs (a, b) VALUES ($1, $1)")
stmt.Execute("same_value")
```

#### SQL Injection Prevention

```go
// UNSAFE - Never do this!
userInput := "'; DROP TABLE users; --"
db.Execute("SELECT * FROM users WHERE name = '" + userInput + "'")

// SAFE - Use prepared statements
stmt, _ := db.Prepare("SELECT * FROM users WHERE name = ?")
rows, _ := stmt.Query(userInput) // Input is properly escaped
```

#### Supported Parameter Types

| Go Type | SQL Type | Example |
|---------|----------|---------|
| `nil` | NULL | `stmt.Exec(nil)` |
| `bool` | TRUE/FALSE | `stmt.Exec(true)` |
| `int`, `int64`, etc. | INTEGER | `stmt.Exec(42)` |
| `float32`, `float64` | REAL | `stmt.Exec(3.14)` |
| `string` | TEXT | `stmt.Exec("hello")` |
| `[]byte` | BLOB | `stmt.Exec([]byte{0x01})` |

#### Placeholder Styles

| Style | Example | Description |
|-------|---------|-------------|
| `?` | `WHERE id = ? AND name = ?` | Positional, MySQL/SQLite style |
| `$N` | `WHERE id = $1 AND name = $2` | Numbered, PostgreSQL style |

**Note:** Cannot mix `?` and `$N` in the same statement.

### Result Structure

```go
type Result struct {
    Columns      []string        // Column names for SELECT
    Rows         [][]interface{} // Row data for SELECT
    RowsAffected int64           // Rows affected for INSERT/UPDATE/DELETE
    LastInsertID int64           // Last inserted ID (if applicable)
    Message      string          // Status message
}
```

### Rows API

```go
type Rows struct {
    Columns() []string           // Get column names
    Next() bool                  // Advance to next row
    Scan(dest ...interface{})    // Scan current row into variables
    Row() []interface{}          // Get current row as slice
    Count() int                  // Total row count
    Close() error                // Close the result set
}
```

### Database Statistics

```go
stats := db.Stats()
fmt.Printf("Tables: %d\n", stats.TableCount)
fmt.Printf("Indexes: %d\n", stats.IndexCount)
fmt.Printf("Buffer Pool Size: %d pages\n", stats.BufferPoolSize)
fmt.Printf("Page Size: %d bytes\n", stats.PageSize)
```

---

## SQL Reference

### CREATE TABLE

```sql
CREATE TABLE table_name (
    column1 datatype [NOT NULL],
    column2 datatype [NOT NULL],
    ...
);
```

**Example:**
```sql
CREATE TABLE employees (
    id INTEGER NOT NULL,
    name TEXT NOT NULL,
    email TEXT,
    salary REAL,
    active BOOLEAN
);
```

### DROP TABLE

```sql
DROP TABLE table_name;
```

### INSERT

```sql
-- Single row
INSERT INTO table_name VALUES (value1, value2, ...);

-- Multiple rows
INSERT INTO table_name VALUES 
    (value1, value2),
    (value3, value4),
    (value5, value6);
```

### SELECT

```sql
-- Basic select
SELECT * FROM table_name;

-- Select specific columns
SELECT column1, column2 FROM table_name;

-- With alias
SELECT column1 AS alias1, column2 AS alias2 FROM table_name;

-- With WHERE clause
SELECT * FROM table_name WHERE condition;

-- With ORDER BY
SELECT * FROM table_name ORDER BY column1 ASC;
SELECT * FROM table_name ORDER BY column1 DESC;

-- With LIMIT
SELECT * FROM table_name LIMIT 10;

-- Combined
SELECT id, name FROM employees 
WHERE salary > 50000 
ORDER BY name 
LIMIT 20;
```

### UPDATE

```sql
-- Update all rows
UPDATE table_name SET column1 = value1;

-- Update with condition
UPDATE table_name SET column1 = value1 WHERE condition;

-- Update multiple columns
UPDATE table_name SET column1 = value1, column2 = value2 WHERE id = 1;

-- Arithmetic update
UPDATE accounts SET balance = balance + 100 WHERE id = 1;
```

### DELETE

```sql
-- Delete all rows
DELETE FROM table_name;

-- Delete with condition
DELETE FROM table_name WHERE condition;
```

### JOIN

```sql
-- INNER JOIN
SELECT u.name, o.product 
FROM users u 
INNER JOIN orders o ON u.id = o.user_id;

-- LEFT JOIN
SELECT u.name, o.product 
FROM users u 
LEFT JOIN orders o ON u.id = o.user_id;
```

### Aggregate Functions

```sql
-- COUNT
SELECT COUNT(*) FROM table_name;
SELECT COUNT(column) FROM table_name;

-- SUM
SELECT SUM(column) FROM table_name;

-- AVG
SELECT AVG(column) FROM table_name;

-- MIN / MAX
SELECT MIN(column), MAX(column) FROM table_name;

-- With GROUP BY
SELECT category, COUNT(*) AS total, SUM(price) AS revenue
FROM products
GROUP BY category;
```

### Transactions

```sql
BEGIN TRANSACTION;
-- or just
BEGIN;

-- ... SQL statements ...

COMMIT;
-- or
ROLLBACK;
```

### WHERE Operators

| Operator | Description |
|----------|-------------|
| `=` | Equal |
| `!=`, `<>` | Not equal |
| `<` | Less than |
| `>` | Greater than |
| `<=` | Less than or equal |
| `>=` | Greater than or equal |
| `AND` | Logical AND |
| `OR` | Logical OR |

---

## Data Types

| SQL Type | Go Type | Description |
|----------|---------|-------------|
| `INTEGER`, `INT` | `int32` | 32-bit integer |
| `BIGINT` | `int64` | 64-bit integer |
| `SMALLINT` | `int16` | 16-bit integer |
| `TINYINT` | `int8` | 8-bit integer |
| `REAL`, `FLOAT`, `DOUBLE` | `float64` | 64-bit float |
| `TEXT`, `VARCHAR`, `CHAR`, `STRING` | `string` | Variable-length text |
| `BLOB`, `BINARY` | `[]byte` | Binary data |
| `BOOLEAN`, `BOOL` | `bool` | Boolean |

---

## Known Limitations

### Current Limitations

1. **Keyword Aliases**: Cannot use SQL keywords as column aliases
   ```sql
   -- Does NOT work:
   SELECT COUNT(*) AS count FROM users;
   
   -- Works:
   SELECT COUNT(*) AS total FROM users;
   SELECT COUNT(*) AS cnt FROM users;
   ```

2. **No RIGHT JOIN**: Only INNER JOIN and LEFT JOIN are supported

3. **No Subqueries**: Nested SELECT statements not supported

4. **No HAVING Clause**: Filter after GROUP BY not implemented

5. **No DISTINCT**: Duplicate elimination not supported

6. **No UNION**: Combining result sets not supported

7. **Single Database File**: No ATTACH database support

### Workarounds

For keyword alias issues, use alternative names:
```sql
-- Instead of 'count', 'sum', 'avg', 'min', 'max'
SELECT COUNT(*) AS total_count FROM users;
SELECT SUM(amount) AS total_sum FROM orders;
SELECT AVG(price) AS average_price FROM products;
```

---

## Examples

### Complete CRUD Example

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/l00pss/citrinedb/engine"
)

func main() {
    // Open database
    db, err := engine.Open("example.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // CREATE
    _, err = db.Execute(`
        CREATE TABLE contacts (
            id INTEGER NOT NULL,
            name TEXT NOT NULL,
            email TEXT,
            phone TEXT
        )
    `)
    if err != nil {
        log.Fatal(err)
    }

    // INSERT
    db.Execute("INSERT INTO contacts VALUES (1, 'Alice', 'alice@example.com', '555-1234')")
    db.Execute("INSERT INTO contacts VALUES (2, 'Bob', 'bob@example.com', '555-5678')")
    db.Execute("INSERT INTO contacts VALUES (3, 'Charlie', 'charlie@example.com', '555-9012')")

    // READ
    rows, _ := db.Query("SELECT * FROM contacts ORDER BY name")
    fmt.Println("All contacts:")
    for rows.Next() {
        var id int
        var name, email, phone string
        rows.Scan(&id, &name, &email, &phone)
        fmt.Printf("  %d: %s <%s> %s\n", id, name, email, phone)
    }

    // UPDATE
    db.Execute("UPDATE contacts SET phone = '555-0000' WHERE id = 1")

    // DELETE
    db.Execute("DELETE FROM contacts WHERE id = 3")

    // Verify
    rows, _ = db.Query("SELECT COUNT(*) AS total FROM contacts")
    if rows.Next() {
        row := rows.Row()
        fmt.Printf("\nRemaining contacts: %v\n", row[0])
    }
}
```

### Transaction Example

```go
func transferMoney(db *engine.DB, fromID, toID int, amount float64) error {
    // Begin transaction
    if _, err := db.Execute("BEGIN TRANSACTION"); err != nil {
        return err
    }

    // Debit from source
    sql := fmt.Sprintf("UPDATE accounts SET balance = balance - %f WHERE id = %d", amount, fromID)
    if _, err := db.Execute(sql); err != nil {
        db.Execute("ROLLBACK")
        return err
    }

    // Credit to destination
    sql = fmt.Sprintf("UPDATE accounts SET balance = balance + %f WHERE id = %d", amount, toID)
    if _, err := db.Execute(sql); err != nil {
        db.Execute("ROLLBACK")
        return err
    }

    // Commit
    if _, err := db.Execute("COMMIT"); err != nil {
        return err
    }

    return nil
}
```

### Aggregate Query Example

```go
func getSalesReport(db *engine.DB) {
    rows, err := db.Query(`
        SELECT 
            category,
            COUNT(*) AS total_products,
            SUM(price) AS total_value,
            AVG(price) AS avg_price,
            MIN(price) AS min_price,
            MAX(price) AS max_price
        FROM products
        GROUP BY category
    `)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Sales Report by Category:")
    fmt.Println("-------------------------")
    
    for rows.Next() {
        row := rows.Row()
        fmt.Printf("Category: %v\n", row[0])
        fmt.Printf("  Products: %v\n", row[1])
        fmt.Printf("  Total Value: $%.2f\n", row[2])
        fmt.Printf("  Avg Price: $%.2f\n", row[3])
        fmt.Printf("  Price Range: $%.2f - $%.2f\n", row[4], row[5])
        fmt.Println()
    }
}
```

### JOIN Example

```go
func getUserOrders(db *engine.DB, userID int) {
    sql := fmt.Sprintf(`
        SELECT u.name, u.email, o.product, o.amount, o.order_date
        FROM users u
        INNER JOIN orders o ON u.id = o.user_id
        WHERE u.id = %d
        ORDER BY o.order_date DESC
    `, userID)

    rows, err := db.Query(sql)
    if err != nil {
        log.Fatal(err)
    }

    for rows.Next() {
        row := rows.Row()
        fmt.Printf("User: %s <%s>\n", row[0], row[1])
        fmt.Printf("  Order: %s - $%.2f (%s)\n", row[2], row[3], row[4])
    }
}
```

### SQL File Execution

**schema.sql:**
```sql
CREATE TABLE users (
    id INTEGER NOT NULL,
    name TEXT NOT NULL,
    email TEXT
);

CREATE TABLE orders (
    id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    product TEXT,
    amount REAL
);
```

**import.sql:**
```sql
INSERT INTO users VALUES (1, 'Alice', 'alice@example.com');
INSERT INTO users VALUES (2, 'Bob', 'bob@example.com');

INSERT INTO orders VALUES (1, 1, 'Widget', 29.99);
INSERT INTO orders VALUES (2, 1, 'Gadget', 49.99);
INSERT INTO orders VALUES (3, 2, 'Widget', 29.99);
```

**Execution:**
```bash
./citrinedb mydata.db <<EOF
.read schema.sql
.read import.sql
SELECT * FROM users;
SELECT * FROM orders;
.quit
EOF
```

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    CitrineDB                            │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │     CLI     │  │   Engine    │  │    Executor     │ │
│  │   (REPL)    │  │    (API)    │  │  (SQL Engine)   │ │
│  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘ │
│         │                │                   │          │
│         └────────────────┼───────────────────┘          │
│                          │                              │
│  ┌───────────────────────┼───────────────────────────┐ │
│  │                    Planner                         │ │
│  │              (Query Optimization)                  │ │
│  └───────────────────────┬───────────────────────────┘ │
│                          │                              │
│  ┌───────────────────────┼───────────────────────────┐ │
│  │                   Storage Layer                    │ │
│  │  ┌─────────┐  ┌──────────┐  ┌─────────────────┐  │ │
│  │  │ Catalog │  │  Buffer  │  │   Transaction   │  │ │
│  │  │         │  │   Pool   │  │   (WAL)         │  │ │
│  │  └────┬────┘  └────┬─────┘  └────────┬────────┘  │ │
│  │       │            │                  │           │ │
│  │  ┌────┴────┐  ┌────┴─────┐  ┌────────┴────────┐  │ │
│  │  │  Table  │  │   Page   │  │      Index      │  │ │
│  │  │ (Heap)  │  │ (Slotted)│  │     (B-Tree)    │  │ │
│  │  └────┬────┘  └────┬─────┘  └────────┬────────┘  │ │
│  │       └────────────┼─────────────────┘           │ │
│  │                    │                              │ │
│  │  ┌─────────────────┴─────────────────────────┐   │ │
│  │  │              Disk Manager                  │   │ │
│  │  │           (File I/O Layer)                 │   │ │
│  │  └────────────────────────────────────────────┘   │ │
│  └───────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## Support

- **GitHub**: https://github.com/l00pss/citrinedb
- **Issues**: https://github.com/l00pss/citrinedb/issues

---

*End of Documentation*
