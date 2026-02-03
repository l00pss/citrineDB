# CitrineDB

<p align="center">
  <img src="logo.png" alt="CitrineDB Logo" width="400">
</p>

<div align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/go-1.25+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat" alt="License"></a>
  <img src="https://img.shields.io/badge/tests-passing-brightgreen?style=flat" alt="Tests">
</div>

<br>

> 🚀 **CitrineDB is embedded SQL database engine written in Go, inspired by SQLite.**

## About

CitrineDB is a lightweight, embedded database engine that implements a complete SQL execution pipeline from parsing to storage. It provides a modular architecture that's easy to understand and extend.

## Features

### Storage Layer
- **Slotted Page Format** - Variable-length record storage in fixed-size pages
- **Buffer Pool** - LRU-based page caching with dirty page tracking
- **Disk Manager** - Low-level file I/O and page allocation
- **B+Tree Index** - Fast key-based lookups using [treego](https://github.com/l00pss/treego)
- **WAL (Write-Ahead Logging)** - Durability and crash recovery using [walrus](https://github.com/l00pss/walrus)
- **Heap File** - Unordered record collection with RID-based access

### Query Processing
- **SQL Parser** - Full SQL parsing with [citrinelexer](https://github.com/l00pss/citrinelexer)
- **Query Planner** - Cost-based optimization with index selection
- **Executor** - Volcano-style iterator model

### SQL Features
- **CRUD Operations** - SELECT, INSERT, UPDATE, DELETE
- **JOINs** - INNER, LEFT, RIGHT, CROSS JOIN
- **Aggregates** - COUNT, SUM, AVG, MIN, MAX with GROUP BY
- **Sorting** - ORDER BY (ASC/DESC), multi-column support
- **Pagination** - LIMIT and OFFSET
- **Filtering** - WHERE with AND/OR/NOT operators

### Transaction Support
- **ACID Transactions** - BEGIN, COMMIT, ROLLBACK
- **Isolation Levels** - Read Uncommitted, Read Committed, Repeatable Read, Serializable
- **Savepoints** - Nested transaction support

## Installation

```bash
go get github.com/l00pss/citrinedb
```

## Quick Start

### Using the CLI

```bash
# Build and run the CLI
go run ./cmd/citrinedb

# Or build the binary
go build -o citrinedb ./cmd/citrinedb
./citrinedb
```

### CLI Commands

```
CitrineDB v0.1.0 - Interactive SQL Shell
Type ".help" for usage hints.

citrinedb> .help
.help     Show this help message
.tables   List all tables
.schema   Show table schemas
.stats    Show database statistics
.quit     Exit the shell

citrinedb> .tables
users
products
orders

citrinedb> .schema users
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  name TEXT,
  email TEXT,
  age INTEGER
);
```

### SQL Examples

```sql
-- Create a table
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT,
  age INTEGER
);

-- Insert data
INSERT INTO users (id, name, email, age) VALUES (1, 'Alice', 'alice@example.com', 30);
INSERT INTO users (id, name, email, age) VALUES (2, 'Bob', 'bob@example.com', 25);
INSERT INTO users (id, name, email, age) VALUES (3, 'Charlie', 'charlie@example.com', 35);

-- Query with filtering and sorting
SELECT name, age FROM users WHERE age > 25 ORDER BY age DESC;

-- Aggregation with GROUP BY
SELECT age, COUNT(*) as count FROM users GROUP BY age;

-- JOIN example
SELECT u.name, o.product 
FROM users u 
INNER JOIN orders o ON u.id = o.user_id;

-- Pagination
SELECT * FROM users ORDER BY id LIMIT 10 OFFSET 20;
```

### Programmatic Usage

```go
package main

import (
    "fmt"
    "github.com/l00pss/citrinedb/engine"
)

func main() {
    // Create a new database engine
    db, err := engine.NewEngine("mydb.db")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Execute SQL statements
    result, err := db.Execute("CREATE TABLE users (id INTEGER, name TEXT)")
    if err != nil {
        panic(err)
    }

    // Insert data
    db.Execute("INSERT INTO users (id, name) VALUES (1, 'Alice')")
    db.Execute("INSERT INTO users (id, name) VALUES (2, 'Bob')")

    // Query data
    result, err = db.Execute("SELECT * FROM users WHERE id = 1")
    if err != nil {
        panic(err)
    }

    // Process results
    for _, row := range result.Rows {
        fmt.Printf("ID: %v, Name: %v\n", row["id"], row["name"])
    }
}
```

### Transaction Example

```go
// Begin a transaction
db.Execute("BEGIN TRANSACTION")

// Perform operations
db.Execute("INSERT INTO accounts (id, balance) VALUES (1, 1000)")
db.Execute("UPDATE accounts SET balance = balance - 100 WHERE id = 1")
db.Execute("INSERT INTO transfers (from_id, amount) VALUES (1, 100)")

// Commit or rollback
db.Execute("COMMIT")
// or: db.Execute("ROLLBACK")
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI (REPL)                              │
├─────────────────────────────────────────────────────────────────┤
│                      SQL Engine                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │    Parser    │→ │   Planner    │→ │   Executor   │          │
│  │ (citrinelexer)  │  (Cost-based) │  │  (Volcano)   │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
├─────────────────────────────────────────────────────────────────┤
│                    Transaction Manager                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │     WAL      │  │  Isolation   │  │  Savepoints  │          │
│  │   (walrus)   │  │    Levels    │  │              │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
├─────────────────────────────────────────────────────────────────┤
│                      Storage Layer                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Catalog    │  │  Heap File   │  │  B+Tree      │          │
│  │   (Schema)   │  │   (Table)    │  │  (Index)     │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ Buffer Pool  │  │    Page      │  │   Record     │          │
│  │    (LRU)     │  │  (Slotted)   │  │ (Serialize)  │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│  ┌──────────────────────────────────────────────────┐          │
│  │              Disk Manager (I/O)                  │          │
│  └──────────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

## Project Structure

```
citrinedb/
├── cmd/
│   └── citrinedb/       # CLI application
│       └── main.go
├── engine/              # SQL execution engine
│   ├── engine.go
│   └── engine_test.go
├── executor/            # Query executors
│   ├── executor.go      # Base executor
│   ├── join.go          # JOIN operations
│   ├── aggregate.go     # Aggregate functions
│   └── sort.go          # ORDER BY, LIMIT
├── planner/             # Query planning
│   ├── planner.go       # Cost-based optimizer
│   └── errors.go
├── storage/
│   ├── buffer/          # Buffer pool management
│   ├── catalog/         # Schema management
│   ├── file/            # Disk I/O
│   ├── index/           # B+Tree implementation
│   ├── page/            # Slotted page format
│   ├── record/          # Record serialization
│   ├── table/           # Heap file storage
│   └── tx/              # Transaction & WAL
└── docs/                # Documentation
    ├── architecture.md
    ├── slotted-page.md
    └── storage-layers-deep-dive.md
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run specific package tests
go test ./storage/buffer -v
go test ./executor -v

# Run with coverage
go test ./... -cover
```

## Documentation

- [Architecture Overview](docs/architecture.md)
- [Slotted Page Format](docs/slotted-page.md)
- [Storage Layers Deep Dive](docs/storage-layers-deep-dive.md)

## Dependencies

| Package | Description |
|---------|-------------|
| [citrinelexer](https://github.com/l00pss/citrinelexer) | SQL lexer and parser |
| [walrus](https://github.com/l00pss/walrus) | Write-Ahead Logging |
| [treego/bplustree](https://github.com/l00pss/treego) | B+Tree implementation |

## Roadmap

- [x] Storage layer (pages, buffer pool, disk manager)
- [x] B+Tree index
- [x] WAL and crash recovery
- [x] Catalog and schema management
- [x] SQL parser integration
- [x] Query planner with cost estimation
- [x] Basic CRUD operations
- [x] JOIN support
- [x] Aggregate functions
- [x] ORDER BY / LIMIT
- [x] Transaction support (BEGIN/COMMIT/ROLLBACK)
- [ ] MVCC (Multi-Version Concurrency Control)
- [ ] Subqueries
- [ ] Views
- [ ] Prepared statements
- [ ] Network protocol (client/server mode)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Author

**Vugar Mammadli** - [@l00pss](https://github.com/l00pss)

---

<div align="center">
  <a href="https://www.buymeacoffee.com/l00pss" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 50px !important;width: 180px !important;" ></a>
</div>