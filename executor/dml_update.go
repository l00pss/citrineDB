package executor

import (
	"fmt"

	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinelexer"
)

// executeUpdate handles UPDATE statements
func (e *Executor) executeUpdate(stmt *citrinelexer.UpdateStatement) (*Result, error) {
	tableName := stmt.Table.Name
	tableInfo, err := e.catalog.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("executor: table '%s' not found", tableName)
	}

	result := NewResult()

	updates := make(map[int]citrinelexer.Expression)
	for _, assign := range stmt.Set {
		colIdx, ok := tableInfo.Schema.FieldIndex(assign.Column.Name)
		if !ok {
			return nil, fmt.Errorf("executor: column '%s' not found", assign.Column.Name)
		}
		updates[colIdx] = assign.Value
	}

	affectedIndexes := make([]*catalog.IndexInfo, 0)
	for _, indexInfo := range tableInfo.Indexes {
		for _, indexCol := range indexInfo.Columns {
			colIdx, _ := tableInfo.Schema.FieldIndex(indexCol)
			if _, affected := updates[colIdx]; affected {
				affectedIndexes = append(affectedIndexes, indexInfo)
				break
			}
		}
	}

	iter := tableInfo.HeapFile.NewIterator()
	defer iter.Close()

	for {
		rec, rid, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("executor: scan error: %w", err)
		}
		if rec == nil {
			break
		}

		if stmt.Where != nil {
			match, err := e.evaluateWhere(stmt.Where, rec, tableInfo.Schema)
			if err != nil {
				return nil, err
			}
			if !match {
				continue
			}
		}

		// Store old index keys before update
		oldKeys := make(map[string]string) // indexName -> oldKey
		for _, indexInfo := range affectedIndexes {
			oldKeys[indexInfo.Name] = e.buildIndexKey(rec, indexInfo.Columns, tableInfo.Schema)
		}

		// Apply updates to record
		for colIdx, expr := range updates {
			field := tableInfo.Schema.Fields[colIdx]
			val, err := e.evaluateExpressionWithContext(expr, field.Type, rec, tableInfo.Schema)
			if err != nil {
				return nil, fmt.Errorf("executor: column '%s': %w", field.Name, err)
			}

			if val.IsNull && !field.Nullable {
				return nil, fmt.Errorf("executor: column '%s' cannot be null", field.Name)
			}

			rec.Set(colIdx, val)
		}

		// Update indexes: remove old key, insert new key
		for _, indexInfo := range affectedIndexes {
			oldKey := oldKeys[indexInfo.Name]
			newKey := e.buildIndexKey(rec, indexInfo.Columns, tableInfo.Schema)

			if oldKey != newKey {
				indexInfo.BTree.Delete(oldKey)
				if err := indexInfo.BTree.Insert(newKey, rid); err != nil {
					return nil, fmt.Errorf("executor: index update failed: %w", err)
				}
			}
		}

		if err := tableInfo.HeapFile.Update(rid, rec); err != nil {
			return nil, fmt.Errorf("executor: update failed: %w", err)
		}

		result.RowsAffected++
	}

	result.Message = fmt.Sprintf("%d row(s) updated", result.RowsAffected)
	return result, nil
}
