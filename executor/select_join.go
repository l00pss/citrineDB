package executor

import (
	"errors"
	"fmt"

	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

// tableContext holds table info for JOIN operations
type tableContext struct {
	tableName    string
	tableInfo    *catalog.TableInfo
	schemaOffset int
}

// joinedRow represents a row from joined tables
type joinedRow struct {
	records map[string]*record.Record // alias -> record
}

// executeSelectJoin handles SELECT with JOIN clauses
func (e *Executor) executeSelectJoin(stmt *citrinelexer.SelectStatement) (*Result, error) {
	if stmt.From == nil || stmt.From.Name == nil {
		return nil, errors.New("executor: SELECT requires FROM clause")
	}

	// Get left table info
	leftTableName := stmt.From.Name.Name
	leftAlias := leftTableName
	if stmt.From.Alias != nil {
		leftAlias = stmt.From.Alias.Name
	}

	leftTableInfo, err := e.catalog.GetTable(leftTableName)
	if err != nil {
		return nil, fmt.Errorf("executor: table '%s' not found", leftTableName)
	}

	// Build table info map: alias -> (tableName, tableInfo, schemaOffset)
	tableMap := make(map[string]*tableContext)
	tableMap[leftAlias] = &tableContext{leftTableName, leftTableInfo, 0}
	tableMap[leftTableName] = tableMap[leftAlias]

	// Process JOIN clauses
	var joinInfos []*struct {
		tableCtx  *tableContext
		alias     string
		joinType  string
		condition *citrinelexer.BinaryExpression
	}

	currentOffset := leftTableInfo.Schema.FieldCount()

	for _, join := range stmt.Joins {
		joinTableName := join.Table.Name.Name
		joinAlias := joinTableName
		if join.Table.Alias != nil {
			joinAlias = join.Table.Alias.Name
		}

		joinTableInfo, err := e.catalog.GetTable(joinTableName)
		if err != nil {
			return nil, fmt.Errorf("executor: table '%s' not found", joinTableName)
		}

		ctx := &tableContext{joinTableName, joinTableInfo, currentOffset}
		tableMap[joinAlias] = ctx
		tableMap[joinTableName] = ctx

		// Parse condition
		var cond *citrinelexer.BinaryExpression
		if join.Condition != nil {
			if be, ok := join.Condition.(*citrinelexer.BinaryExpression); ok {
				cond = be
			}
		}

		joinInfos = append(joinInfos, &struct {
			tableCtx  *tableContext
			alias     string
			joinType  string
			condition *citrinelexer.BinaryExpression
		}{ctx, joinAlias, join.Type, cond})

		currentOffset += joinTableInfo.Schema.FieldCount()
	}

	// Get all left table rows
	var leftRows []*record.Record
	leftIter := leftTableInfo.HeapFile.NewIterator()
	defer leftIter.Close()

	for {
		rec, _, err := leftIter.Next()
		if err != nil {
			return nil, fmt.Errorf("executor: scan error: %w", err)
		}
		if rec == nil {
			break
		}
		leftRows = append(leftRows, rec)
	}

	// Build joined rows
	joinedRows := make([]*joinedRow, 0)

	// Start with left table rows
	for _, leftRec := range leftRows {
		jr := &joinedRow{records: make(map[string]*record.Record)}
		jr.records[leftAlias] = leftRec
		joinedRows = append(joinedRows, jr)
	}

	// Apply each JOIN
	for _, ji := range joinInfos {
		// Get all rows from join table
		var joinTableRows []*record.Record
		joinIter := ji.tableCtx.tableInfo.HeapFile.NewIterator()

		for {
			rec, _, err := joinIter.Next()
			if err != nil {
				joinIter.Close()
				return nil, fmt.Errorf("executor: scan error: %w", err)
			}
			if rec == nil {
				break
			}
			joinTableRows = append(joinTableRows, rec)
		}
		joinIter.Close()

		// Perform join
		newJoinedRows := make([]*joinedRow, 0)

		// Track which right rows have been matched (for RIGHT/FULL JOIN)
		rightMatched := make([]bool, len(joinTableRows))

		for _, jr := range joinedRows {
			matched := false
			for i, rightRec := range joinTableRows {
				// Check join condition
				if ji.condition != nil {
					match, err := e.evaluateJoinCondition(ji.condition, jr.records, rightRec, ji.alias, tableMap, leftTableInfo.Schema, ji.tableCtx.tableInfo.Schema)
					if err != nil {
						return nil, err
					}
					if !match {
						continue
					}
				}

				// Match found - create new joined row
				newJR := &joinedRow{records: make(map[string]*record.Record)}
				for k, v := range jr.records {
					newJR.records[k] = v
				}
				newJR.records[ji.alias] = rightRec
				newJoinedRows = append(newJoinedRows, newJR)
				matched = true
				rightMatched[i] = true
			}

			// For LEFT JOIN or FULL JOIN, include unmatched left rows
			if !matched && (ji.joinType == "LEFT" || ji.joinType == "FULL") {
				newJR := &joinedRow{records: make(map[string]*record.Record)}
				for k, v := range jr.records {
					newJR.records[k] = v
				}
				newJR.records[ji.alias] = nil // NULL for right side
				newJoinedRows = append(newJoinedRows, newJR)
			}
		}

		// For RIGHT JOIN or FULL JOIN, include unmatched right rows
		if ji.joinType == "RIGHT" || ji.joinType == "FULL" {
			for i, rightRec := range joinTableRows {
				if !rightMatched[i] {
					newJR := &joinedRow{records: make(map[string]*record.Record)}
					// Set all left-side aliases to nil
					for alias := range tableMap {
						if alias != ji.alias {
							newJR.records[alias] = nil
						}
					}
					newJR.records[ji.alias] = rightRec
					newJoinedRows = append(newJoinedRows, newJR)
				}
			}
		}

		joinedRows = newJoinedRows
	}

	// Apply WHERE filter to joined rows
	if stmt.Where != nil {
		filteredRows := make([]*joinedRow, 0)
		for _, jr := range joinedRows {
			match, err := e.evaluateJoinedRowWhere(stmt.Where, jr, tableMap)
			if err != nil {
				return nil, err
			}
			if match {
				filteredRows = append(filteredRows, jr)
			}
		}
		joinedRows = filteredRows
	}

	// Build result
	result := NewResult()

	// Determine columns to select
	for _, field := range stmt.Fields {
		switch f := field.(type) {
		case *citrinelexer.QualifiedIdentifier:
			result.Columns = append(result.Columns, f.Column)
		case *citrinelexer.Identifier:
			result.Columns = append(result.Columns, f.Name)
		case *citrinelexer.AliasedExpression:
			if f.Alias != "" {
				result.Columns = append(result.Columns, f.Alias)
			}
		}
	}

	// Extract values
	for _, jr := range joinedRows {
		row := make([]interface{}, 0)

		for _, field := range stmt.Fields {
			switch f := field.(type) {
			case *citrinelexer.QualifiedIdentifier:
				tableCtx, ok := tableMap[f.Table]
				if !ok {
					return nil, fmt.Errorf("executor: unknown table or alias '%s'", f.Table)
				}

				rec := jr.records[f.Table]
				if rec == nil {
					row = append(row, nil)
					continue
				}

				colIdx, ok := tableCtx.tableInfo.Schema.FieldIndex(f.Column)
				if !ok {
					return nil, fmt.Errorf("executor: column '%s' not found in '%s'", f.Column, f.Table)
				}

				val, _ := rec.Get(colIdx)
				row = append(row, valueToInterface(val))

			case *citrinelexer.Identifier:
				// Try to find column in any table
				var found bool
				for alias, rec := range jr.records {
					if rec == nil {
						continue
					}
					tableCtx := tableMap[alias]
					colIdx, ok := tableCtx.tableInfo.Schema.FieldIndex(f.Name)
					if ok {
						val, _ := rec.Get(colIdx)
						row = append(row, valueToInterface(val))
						found = true
						break
					}
				}
				if !found {
					row = append(row, nil)
				}
			}
		}

		result.Rows = append(result.Rows, row)
	}

	// Apply DISTINCT if specified
	if stmt.Distinct {
		result.Rows = removeDuplicateRows(result.Rows)
	}

	return result, nil
}

// evaluateJoinCondition evaluates a JOIN ON condition
func (e *Executor) evaluateJoinCondition(cond *citrinelexer.BinaryExpression, leftRecords map[string]*record.Record, rightRec *record.Record, rightAlias string, tableMap map[string]*tableContext, leftSchema *record.Schema, rightSchema *record.Schema) (bool, error) {
	// Get left value
	var leftVal interface{}
	if qi, ok := cond.Left.(*citrinelexer.QualifiedIdentifier); ok {
		if rec, exists := leftRecords[qi.Table]; exists && rec != nil {
			tableCtx := tableMap[qi.Table]
			colIdx, _ := tableCtx.tableInfo.Schema.FieldIndex(qi.Column)
			val, _ := rec.Get(colIdx)
			leftVal = valueToInterface(val)
		} else if qi.Table == rightAlias && rightRec != nil {
			colIdx, _ := rightSchema.FieldIndex(qi.Column)
			val, _ := rightRec.Get(colIdx)
			leftVal = valueToInterface(val)
		}
	}

	// Get right value
	var rightVal interface{}
	if qi, ok := cond.Right.(*citrinelexer.QualifiedIdentifier); ok {
		if rec, exists := leftRecords[qi.Table]; exists && rec != nil {
			tableCtx := tableMap[qi.Table]
			colIdx, _ := tableCtx.tableInfo.Schema.FieldIndex(qi.Column)
			val, _ := rec.Get(colIdx)
			rightVal = valueToInterface(val)
		} else if qi.Table == rightAlias && rightRec != nil {
			colIdx, _ := rightSchema.FieldIndex(qi.Column)
			val, _ := rightRec.Get(colIdx)
			rightVal = valueToInterface(val)
		}
	}

	return e.compare(leftVal, rightVal, cond.Operator)
}

// evaluateJoinedRowWhere evaluates WHERE condition on a joined row
func (e *Executor) evaluateJoinedRowWhere(expr citrinelexer.Expression, jr *joinedRow, tableMap map[string]*tableContext) (bool, error) {
	switch ex := expr.(type) {
	case *citrinelexer.BinaryExpression:
		return e.evaluateJoinedRowBinaryExpr(ex, jr, tableMap)
	case *citrinelexer.BooleanLiteral:
		return ex.Value, nil
	case *citrinelexer.InExpression:
		return e.evaluateJoinedRowIn(ex, jr, tableMap)
	case *citrinelexer.BetweenExpression:
		return e.evaluateJoinedRowBetween(ex, jr, tableMap)
	default:
		return false, ErrInvalidExpression
	}
}

// evaluateJoinedRowBinaryExpr evaluates a binary expression on joined row
func (e *Executor) evaluateJoinedRowBinaryExpr(expr *citrinelexer.BinaryExpression, jr *joinedRow, tableMap map[string]*tableContext) (bool, error) {
	switch expr.Operator {
	case "AND":
		left, err := e.evaluateJoinedRowWhere(expr.Left, jr, tableMap)
		if err != nil {
			return false, err
		}
		if !left {
			return false, nil
		}
		return e.evaluateJoinedRowWhere(expr.Right, jr, tableMap)
	case "OR":
		left, err := e.evaluateJoinedRowWhere(expr.Left, jr, tableMap)
		if err != nil {
			return false, err
		}
		if left {
			return true, nil
		}
		return e.evaluateJoinedRowWhere(expr.Right, jr, tableMap)
	}

	leftVal := e.getJoinedRowValue(expr.Left, jr, tableMap)
	rightVal := e.getJoinedRowValue(expr.Right, jr, tableMap)

	return e.compare(leftVal, rightVal, expr.Operator)
}

// getJoinedRowValue extracts a value from joined row based on expression
func (e *Executor) getJoinedRowValue(expr citrinelexer.Expression, jr *joinedRow, tableMap map[string]*tableContext) interface{} {
	switch ex := expr.(type) {
	case *citrinelexer.QualifiedIdentifier:
		rec := jr.records[ex.Table]
		if rec == nil {
			return nil
		}
		tableCtx := tableMap[ex.Table]
		if tableCtx == nil {
			return nil
		}
		colIdx, ok := tableCtx.tableInfo.Schema.FieldIndex(ex.Column)
		if !ok {
			return nil
		}
		val, _ := rec.Get(colIdx)
		return valueToInterface(val)

	case *citrinelexer.Identifier:
		// Try to find in any table
		for alias, rec := range jr.records {
			if rec == nil {
				continue
			}
			tableCtx := tableMap[alias]
			if tableCtx == nil {
				continue
			}
			colIdx, ok := tableCtx.tableInfo.Schema.FieldIndex(ex.Name)
			if ok {
				val, _ := rec.Get(colIdx)
				return valueToInterface(val)
			}
		}
		return nil

	case *citrinelexer.StringLiteral:
		return ex.Value
	case *citrinelexer.NumberLiteral:
		return ex.Value
	case *citrinelexer.BooleanLiteral:
		return ex.Value
	case *citrinelexer.NullLiteral:
		return nil
	default:
		return e.expressionToInterface(expr)
	}
}

// evaluateJoinedRowIn evaluates IN expression on joined row
func (e *Executor) evaluateJoinedRowIn(expr *citrinelexer.InExpression, jr *joinedRow, tableMap map[string]*tableContext) (bool, error) {
	val := e.getJoinedRowValue(expr.Expr, jr, tableMap)
	if val == nil {
		return expr.Not, nil // NULL IN (...) is false, NULL NOT IN (...) is true
	}

	valStr := fmt.Sprintf("%v", val)

	for _, item := range expr.Values {
		itemVal := e.getJoinedRowValue(item, jr, tableMap)
		itemStr := fmt.Sprintf("%v", itemVal)
		if valStr == itemStr {
			return !expr.Not, nil
		}
	}

	return expr.Not, nil
}

// evaluateJoinedRowBetween evaluates BETWEEN expression on joined row
func (e *Executor) evaluateJoinedRowBetween(expr *citrinelexer.BetweenExpression, jr *joinedRow, tableMap map[string]*tableContext) (bool, error) {
	val := e.getJoinedRowValue(expr.Expr, jr, tableMap)
	low := e.getJoinedRowValue(expr.Low, jr, tableMap)
	high := e.getJoinedRowValue(expr.High, jr, tableMap)

	result := compareNumeric(val, low) >= 0 && compareNumeric(val, high) <= 0

	if expr.Not {
		return !result, nil
	}
	return result, nil
}
