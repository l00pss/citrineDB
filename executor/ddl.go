package executor

import (
	"fmt"

	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

// executeCreateTable handles CREATE TABLE statements
func (e *Executor) executeCreateTable(stmt *citrinelexer.CreateTableStatement) (*Result, error) {
	tableName := stmt.Table.Name

	fields := make([]record.Field, 0, len(stmt.Columns))
	for _, col := range stmt.Columns {
		fieldType, err := mapDataType(col.Type)
		if err != nil {
			return nil, fmt.Errorf("executor: column '%s': %w", col.Name.Name, err)
		}

		nullable := true
		for _, constraint := range col.Constraints {
			if _, ok := constraint.(*citrinelexer.NotNullConstraint); ok {
				nullable = false
				break
			}
		}

		field := record.Field{
			Name:     col.Name.Name,
			Type:     fieldType,
			Nullable: nullable,
		}

		if fieldType == record.FieldTypeString {
			field.MaxLen = 255
		}

		fields = append(fields, field)
	}

	schema := record.NewSchema(fields)
	_, err := e.catalog.CreateTable(tableName, schema)
	if err != nil {
		return nil, fmt.Errorf("executor: failed to create table: %w", err)
	}

	result := NewResult()
	result.Message = fmt.Sprintf("Table '%s' created successfully", tableName)
	return result, nil
}

// executeCreateIndex handles CREATE INDEX statements
func (e *Executor) executeCreateIndex(stmt *citrinelexer.CreateIndexStatement) (*Result, error) {
	indexName := stmt.Name.Name
	tableName := stmt.Table.Name

	// Check if index already exists
	if stmt.IfNotExists {
		if _, err := e.catalog.GetIndex(indexName); err == nil {
			result := NewResult()
			result.Message = fmt.Sprintf("Index '%s' already exists", indexName)
			return result, nil
		}
	}

	// Extract column names
	columns := make([]string, len(stmt.Columns))
	for i, col := range stmt.Columns {
		columns[i] = col.Column.Name
	}

	// Get table info to validate columns and populate index
	tableInfo, err := e.catalog.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("executor: table '%s' not found", tableName)
	}

	// Validate all columns exist
	for _, colName := range columns {
		if _, ok := tableInfo.Schema.FieldIndex(colName); !ok {
			return nil, fmt.Errorf("executor: column '%s' not found in table '%s'", colName, tableName)
		}
	}

	// Create index in catalog
	indexInfo, err := e.catalog.CreateIndex(indexName, tableName, columns, stmt.Unique)
	if err != nil {
		return nil, fmt.Errorf("executor: failed to create index: %w", err)
	}

	// Populate index with existing data
	iter := tableInfo.HeapFile.NewIterator()
	defer iter.Close()

	recordCount := 0
	for {
		rec, rid, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("executor: scan error while building index: %w", err)
		}
		if rec == nil {
			break
		}

		key := e.buildIndexKey(rec, columns, tableInfo.Schema)
		if err := indexInfo.BTree.Insert(key, rid); err != nil {
			// If unique constraint violated, drop the index and return error
			if stmt.Unique {
				_ = e.catalog.DropIndex(indexName)
				return nil, fmt.Errorf("executor: unique constraint violated for key '%s'", key)
			}
			return nil, fmt.Errorf("executor: failed to insert into index: %w", err)
		}
		recordCount++
	}

	result := NewResult()
	if stmt.Unique {
		result.Message = fmt.Sprintf("Unique index '%s' created on %s(%s) with %d entries",
			indexName, tableName, joinColumns(columns), recordCount)
	} else {
		result.Message = fmt.Sprintf("Index '%s' created on %s(%s) with %d entries",
			indexName, tableName, joinColumns(columns), recordCount)
	}
	return result, nil
}

// executeDropIndex handles DROP INDEX statements
func (e *Executor) executeDropIndex(stmt *citrinelexer.DropIndexStatement) (*Result, error) {
	indexName := stmt.Name.Name

	// Check if index exists
	if stmt.IfExists {
		if _, err := e.catalog.GetIndex(indexName); err != nil {
			result := NewResult()
			result.Message = fmt.Sprintf("Index '%s' does not exist", indexName)
			return result, nil
		}
	}

	if err := e.catalog.DropIndex(indexName); err != nil {
		return nil, fmt.Errorf("executor: failed to drop index: %w", err)
	}

	result := NewResult()
	result.Message = fmt.Sprintf("Index '%s' dropped successfully", indexName)
	return result, nil
}

// joinColumns joins column names with comma for display
func joinColumns(columns []string) string {
	result := ""
	for i, col := range columns {
		if i > 0 {
			result += ", "
		}
		result += col
	}
	return result
}
