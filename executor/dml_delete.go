package executor

import (
	"fmt"

	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

// executeDelete handles DELETE statements
func (e *Executor) executeDelete(stmt *citrinelexer.DeleteStatement) (*Result, error) {
	tableName := stmt.From.Name
	tableInfo, err := e.catalog.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("executor: table '%s' not found", tableName)
	}

	result := NewResult()

	type deleteInfo struct {
		rid       record.RecordID
		indexKeys map[string]string // indexName -> key
	}
	var toDelete []deleteInfo

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

		// Store index keys for this record before deletion
		info := deleteInfo{
			rid:       rid,
			indexKeys: make(map[string]string),
		}
		for _, indexInfo := range tableInfo.Indexes {
			info.indexKeys[indexInfo.Name] = e.buildIndexKey(rec, indexInfo.Columns, tableInfo.Schema)
		}
		toDelete = append(toDelete, info)
	}

	// Delete records and update indexes
	for _, info := range toDelete {
		// Remove from all indexes first
		for indexName, key := range info.indexKeys {
			if indexInfo, ok := tableInfo.Indexes[indexName]; ok {
				indexInfo.BTree.Delete(key)
			}
		}

		if err := tableInfo.HeapFile.Delete(info.rid); err != nil {
			return nil, fmt.Errorf("executor: delete failed: %w", err)
		}
		result.RowsAffected++
	}

	result.Message = fmt.Sprintf("%d row(s) deleted", result.RowsAffected)
	return result, nil
}
