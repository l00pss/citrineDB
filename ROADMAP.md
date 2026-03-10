# CitrineDB Roadmap

This document outlines the planned features and improvements for CitrineDB.

---

## v0.2.0 - Query Enhancements

| Feature | Description | Status |
|---------|-------------|--------|
| `CREATE INDEX` | User-defined indexes on columns | 🔲 Planned |
| `DISTINCT` | `SELECT DISTINCT column FROM table` | 🔲 Planned |
| `LIKE` / `GLOB` | Pattern matching in WHERE clause | 🔲 Planned |
| `IN` / `NOT IN` | `WHERE id IN (1, 2, 3)` | 🔲 Planned |
| `BETWEEN` | `WHERE age BETWEEN 20 AND 30` | 🔲 Planned |
| `RIGHT JOIN` | Right outer join support | 🔲 Planned |
| `FULL OUTER JOIN` | Full outer join support | 🔲 Planned |

---

## v0.3.0 - Constraints & Integrity

| Feature | Description | Status |
|---------|-------------|--------|
| `AUTOINCREMENT` | Auto-incrementing primary keys | 🔲 Planned |
| `UNIQUE` | Unique constraint on columns | 🔲 Planned |
| `DEFAULT` | Default values for columns | 🔲 Planned |
| `CHECK` | Check constraints | 🔲 Planned |
| `FOREIGN KEY` | Referential integrity | 🔲 Planned |
| `HAVING` | Filter aggregate results | 🔲 Planned |

---

## v0.4.0 - Subqueries & Expressions

| Feature | Description | Status |
|---------|-------------|--------|
| Subqueries | `SELECT * FROM (SELECT ...)` | 🔲 Planned |
| `EXISTS` | `WHERE EXISTS (SELECT ...)` | 🔲 Planned |
| `CASE WHEN` | Conditional expressions | 🔲 Planned |
| `COALESCE` | Null handling | 🔲 Planned |
| `NULLIF` | Null comparison | 🔲 Planned |
| `UNION` / `UNION ALL` | Combine result sets | 🔲 Planned |

---

## v0.5.0 - Built-in Functions

### String Functions
- `UPPER(str)`, `LOWER(str)`
- `LENGTH(str)`, `SUBSTR(str, start, len)`
- `TRIM(str)`, `LTRIM(str)`, `RTRIM(str)`
- `REPLACE(str, from, to)`
- `CONCAT(str1, str2, ...)`

### Math Functions
- `ABS(x)`, `ROUND(x, n)`
- `CEIL(x)`, `FLOOR(x)`
- `MOD(x, y)`, `POWER(x, y)`
- `RANDOM()`

### Date/Time Functions
- `NOW()`, `DATE()`, `TIME()`
- `YEAR(date)`, `MONTH(date)`, `DAY(date)`
- `DATE_ADD()`, `DATE_SUB()`
- `STRFTIME(format, date)`

---

## v0.6.0 - Views & Virtual Tables

| Feature | Description | Status |
|---------|-------------|--------|
| `CREATE VIEW` | Stored queries | 🔲 Planned |
| `DROP VIEW` | Remove views | 🔲 Planned |
| `ALTER TABLE` | Modify table schema | 🔲 Planned |
| `RENAME TABLE` | Rename existing tables | 🔲 Planned |

---

## v1.0.0 - Production Ready

| Feature | Description | Status |
|---------|-------------|--------|
| `EXPLAIN` | Query execution plan | 🔲 Planned |
| Query Cache | Result caching | 🔲 Planned |
| Connection Pooling | Multi-client support | 🔲 Planned |
| Hot Backup | Online backup/restore | 🔲 Planned |
| Vacuum/Compact | Page defragmentation | 🔲 Planned |
| Triggers | Event-based execution | 🔲 Planned |
| Stored Procedures | User-defined procedures | 🔲 Planned |

---

## Completed Features (v0.1.0) ✅

- [x] SQL Parser (citrinelexer)
- [x] Prepared Statements
- [x] Transactions (BEGIN/COMMIT/ROLLBACK)
- [x] WAL-based durability
- [x] INNER JOIN, LEFT JOIN
- [x] Aggregates (COUNT, SUM, AVG, MIN, MAX)
- [x] GROUP BY, ORDER BY, LIMIT
- [x] Table and column aliases
- [x] NULL value support
- [x] B-Tree indexing
- [x] Buffer Pool with LRU
- [x] Slotted Page storage

---

## Contributing

Contributions are welcome! If you'd like to work on any of these features:

1. Check if an issue exists for the feature
2. Open a new issue if not
3. Fork the repository
4. Create a feature branch
5. Submit a pull request

---

*Last updated: February 4, 2026*
