package executor

import (
	"errors"
	"fmt"

	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

// executeSelect handles SELECT statements
func (e *Executor) executeSelect(stmt *citrinelexer.SelectStatement) (*Result, error) {
	// Check if this is a JOIN query
	if len(stmt.Joins) > 0 {
		return e.executeSelectJoin(stmt)
	}

	// Handle TableRef type for From
	var tableName string
	var tableAlias string

	if stmt.From == nil {
		// Check if this is a simple aggregate query
		if len(stmt.Fields) > 0 {
			if _, ok := stmt.Fields[0].(*citrinelexer.FunctionCall); ok {
				return nil, errors.New("executor: SELECT requires FROM clause")
			}
		}
		return nil, errors.New("executor: SELECT requires FROM clause")
	}

	// Extract table name from TableRef
	if stmt.From.Name != nil {
		tableName = stmt.From.Name.Name
	} else {
		return nil, errors.New("executor: SELECT requires FROM clause")
	}

	// Extract alias if present
	if stmt.From.Alias != nil {
		tableAlias = stmt.From.Alias.Name
	}

	// Build alias map for qualified column resolution
	aliasMap := make(map[string]string) // alias -> tableName
	if tableAlias != "" {
		aliasMap[tableAlias] = tableName
	}
	aliasMap[tableName] = tableName // table name also works as alias

	tableInfo, err := e.catalog.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("executor: table '%s' not found", tableName)
	}

	// Check if we have aggregate functions
	hasAggregate := false
	for _, field := range stmt.Fields {
		if _, ok := field.(*citrinelexer.FunctionCall); ok {
			hasAggregate = true
			break
		}
	}

	if hasAggregate {
		return e.executeSelectAggregate(stmt, tableInfo)
	}

	selectAll := false
	var columnNames []string

	for _, field := range stmt.Fields {
		switch f := field.(type) {
		case *citrinelexer.Identifier:
			if f.Name == "*" {
				selectAll = true
			} else {
				columnNames = append(columnNames, f.Name)
			}
		case *citrinelexer.QualifiedIdentifier:
			// Handle table.column or alias.column
			tableRef := f.Table
			colName := f.Column

			// Verify table reference matches our table or alias
			if _, ok := aliasMap[tableRef]; !ok {
				return nil, fmt.Errorf("executor: unknown table or alias '%s'", tableRef)
			}
			columnNames = append(columnNames, colName)
		}
	}

	if selectAll {
		columnNames = make([]string, 0, tableInfo.Schema.FieldCount())
		for _, f := range tableInfo.Schema.Fields {
			columnNames = append(columnNames, f.Name)
		}
	}

	result := NewResult()
	result.Columns = columnNames

	colIndexes := make([]int, len(columnNames))
	for i, name := range columnNames {
		idx, ok := tableInfo.Schema.FieldIndex(name)
		if !ok {
			return nil, fmt.Errorf("executor: column '%s' not found", name)
		}
		colIndexes[i] = idx
	}

	iter := tableInfo.HeapFile.NewIterator()
	defer iter.Close()

	for {
		rec, _, err := iter.Next()
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

		row := make([]interface{}, len(colIndexes))
		for i, colIdx := range colIndexes {
			val, err := rec.Get(colIdx)
			if err != nil {
				row[i] = nil
				continue
			}
			row[i] = valueToInterface(val)
		}
		result.Rows = append(result.Rows, row)
	}

	// Apply DISTINCT if specified
	if stmt.Distinct {
		result.Rows = removeDuplicateRows(result.Rows)
	}

	return result, nil
}

// executeSelectAggregate handles SELECT with aggregate functions
func (e *Executor) executeSelectAggregate(stmt *citrinelexer.SelectStatement, tableInfo *catalog.TableInfo) (*Result, error) {
	result := NewResult()

	// Collect all rows first
	var rows []*record.Record
	iter := tableInfo.HeapFile.NewIterator()
	defer iter.Close()

	for {
		rec, _, err := iter.Next()
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
		rows = append(rows, rec)
	}

	// Process each field
	var values []interface{}
	for _, field := range stmt.Fields {
		switch f := field.(type) {
		case *citrinelexer.FunctionCall:
			val, colName, err := e.evaluateAggregateFunc(f, rows, tableInfo.Schema)
			if err != nil {
				return nil, err
			}
			result.Columns = append(result.Columns, colName)
			values = append(values, val)
		case *citrinelexer.Identifier:
			result.Columns = append(result.Columns, f.Name)
			if len(rows) > 0 {
				colIdx, ok := tableInfo.Schema.FieldIndex(f.Name)
				if ok {
					v, _ := rows[0].Get(colIdx)
					values = append(values, valueToInterface(v))
				} else {
					values = append(values, nil)
				}
			} else {
				values = append(values, nil)
			}
		}
	}

	if len(values) > 0 {
		result.Rows = append(result.Rows, values)
	}

	return result, nil
}

// evaluateAggregateFunc evaluates an aggregate function
func (e *Executor) evaluateAggregateFunc(f *citrinelexer.FunctionCall, rows []*record.Record, schema *record.Schema) (interface{}, string, error) {
	funcName := f.Name
	colName := funcName + "(*)"

	// Get column name if specified
	var targetCol string
	if len(f.Args) > 0 {
		if ident, ok := f.Args[0].(*citrinelexer.Identifier); ok {
			targetCol = ident.Name
			if targetCol != "*" {
				colName = funcName + "(" + targetCol + ")"
			}
		}
	}

	switch funcName {
	case "COUNT":
		return int64(len(rows)), colName, nil

	case "SUM":
		if targetCol == "" || targetCol == "*" {
			return nil, colName, errors.New("executor: SUM requires a column name")
		}
		colIdx, ok := schema.FieldIndex(targetCol)
		if !ok {
			return nil, colName, fmt.Errorf("executor: column '%s' not found", targetCol)
		}
		var sum float64
		for _, rec := range rows {
			val, _ := rec.Get(colIdx)
			sum += toFloat64(valueToInterface(val))
		}
		return sum, colName, nil

	case "AVG":
		if targetCol == "" || targetCol == "*" {
			return nil, colName, errors.New("executor: AVG requires a column name")
		}
		colIdx, ok := schema.FieldIndex(targetCol)
		if !ok {
			return nil, colName, fmt.Errorf("executor: column '%s' not found", targetCol)
		}
		if len(rows) == 0 {
			return nil, colName, nil
		}
		var sum float64
		for _, rec := range rows {
			val, _ := rec.Get(colIdx)
			sum += toFloat64(valueToInterface(val))
		}
		return sum / float64(len(rows)), colName, nil

	case "MIN":
		if targetCol == "" || targetCol == "*" {
			return nil, colName, errors.New("executor: MIN requires a column name")
		}
		colIdx, ok := schema.FieldIndex(targetCol)
		if !ok {
			return nil, colName, fmt.Errorf("executor: column '%s' not found", targetCol)
		}
		if len(rows) == 0 {
			return nil, colName, nil
		}
		minVal := toFloat64(valueToInterface(func() record.Value { v, _ := rows[0].Get(colIdx); return v }()))
		for _, rec := range rows {
			val, _ := rec.Get(colIdx)
			v := toFloat64(valueToInterface(val))
			if v < minVal {
				minVal = v
			}
		}
		return minVal, colName, nil

	case "MAX":
		if targetCol == "" || targetCol == "*" {
			return nil, colName, errors.New("executor: MAX requires a column name")
		}
		colIdx, ok := schema.FieldIndex(targetCol)
		if !ok {
			return nil, colName, fmt.Errorf("executor: column '%s' not found", targetCol)
		}
		if len(rows) == 0 {
			return nil, colName, nil
		}
		maxVal := toFloat64(valueToInterface(func() record.Value { v, _ := rows[0].Get(colIdx); return v }()))
		for _, rec := range rows {
			val, _ := rec.Get(colIdx)
			v := toFloat64(valueToInterface(val))
			if v > maxVal {
				maxVal = v
			}
		}
		return maxVal, colName, nil

	default:
		return nil, colName, fmt.Errorf("executor: unsupported function '%s'", funcName)
	}
}

// removeDuplicateRows removes duplicate rows from result set for DISTINCT
func removeDuplicateRows(rows [][]interface{}) [][]interface{} {
	if len(rows) == 0 {
		return rows
	}

	seen := make(map[string]bool)
	result := make([][]interface{}, 0, len(rows))

	for _, row := range rows {
		key := rowToKey(row)
		if !seen[key] {
			seen[key] = true
			result = append(result, row)
		}
	}

	return result
}

// rowToKey converts a row to a string key for duplicate detection
func rowToKey(row []interface{}) string {
	key := ""
	for i, val := range row {
		if i > 0 {
			key += "|"
		}
		if val == nil {
			key += "<NULL>"
		} else {
			key += fmt.Sprintf("%v", val)
		}
	}
	return key
}
