package executor

import (
	"fmt"

	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

// executeInsert handles INSERT statements
func (e *Executor) executeInsert(stmt *citrinelexer.InsertStatement) (*Result, error) {
	tableName := stmt.Table.Name
	tableInfo, err := e.catalog.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("executor: table '%s' not found", tableName)
	}

	result := NewResult()

	for _, values := range stmt.Values {
		if len(values) != tableInfo.Schema.FieldCount() {
			return nil, fmt.Errorf("executor: expected %d values, got %d",
				tableInfo.Schema.FieldCount(), len(values))
		}

		rec := record.NewRecord(tableInfo.Schema)

		for i, expr := range values {
			field := tableInfo.Schema.Fields[i]
			val, err := e.expressionToValue(expr, field.Type)
			if err != nil {
				return nil, fmt.Errorf("executor: column '%s': %w", field.Name, err)
			}

			if val.IsNull && !field.Nullable {
				return nil, fmt.Errorf("executor: column '%s' cannot be null", field.Name)
			}

			rec.Set(i, val)
		}

		rid, err := tableInfo.HeapFile.Insert(rec)
		if err != nil {
			return nil, fmt.Errorf("executor: insert failed: %w", err)
		}

		for _, indexInfo := range tableInfo.Indexes {
			key := e.buildIndexKey(rec, indexInfo.Columns, tableInfo.Schema)
			if err := indexInfo.BTree.Insert(key, rid); err != nil {
				return nil, fmt.Errorf("executor: index update failed: %w", err)
			}
		}

		result.RowsAffected++
	}

	result.Message = fmt.Sprintf("%d row(s) inserted", result.RowsAffected)
	return result, nil
}
