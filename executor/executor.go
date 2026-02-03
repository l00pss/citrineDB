package executor

import (
	"errors"
	"fmt"
	"time"

	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinedb/storage/tx"
	"github.com/l00pss/citrinelexer"
	"github.com/l00pss/walrus"
)

var (
	ErrUnsupportedStatement = errors.New("executor: unsupported statement type")
	ErrInvalidExpression    = errors.New("executor: invalid expression")
	ErrTypeMismatch         = errors.New("executor: type mismatch")
	ErrNullConstraint       = errors.New("executor: null constraint violation")
	ErrInvalidDataType      = errors.New("executor: invalid data type")
	ErrNoActiveTransaction  = errors.New("executor: no active transaction")
	ErrTransactionActive    = errors.New("executor: transaction already active")
)

// Result represents the result of a query execution
type Result struct {
	Columns      []string
	Rows         [][]interface{}
	RowsAffected int64
	LastInsertID int64
	Message      string
}

// NewResult creates a new empty result
func NewResult() *Result {
	return &Result{
		Columns: make([]string, 0),
		Rows:    make([][]interface{}, 0),
	}
}

// Executor handles SQL statement execution
type Executor struct {
	catalog    *catalog.Catalog
	wal        *tx.WALManager
	activeTxID walrus.TransactionID
	inTx       bool
}

// NewExecutor creates a new executor
func NewExecutor(cat *catalog.Catalog) *Executor {
	return &Executor{
		catalog: cat,
	}
}

// NewExecutorWithWAL creates a new executor with WAL support
func NewExecutorWithWAL(cat *catalog.Catalog, wal *tx.WALManager) *Executor {
	return &Executor{
		catalog: cat,
		wal:     wal,
	}
}

// Execute parses and executes a SQL statement
func (e *Executor) Execute(sql string) (*Result, error) {
	stmt, err := citrinelexer.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("executor: parse error: %w", err)
	}

	return e.ExecuteStatement(stmt)
}

// ExecuteStatement executes a parsed statement
func (e *Executor) ExecuteStatement(stmt citrinelexer.Statement) (*Result, error) {
	switch s := stmt.(type) {
	case *citrinelexer.CreateTableStatement:
		return e.executeCreateTable(s)
	case *citrinelexer.SelectStatement:
		return e.executeSelect(s)
	case *citrinelexer.InsertStatement:
		return e.executeInsert(s)
	case *citrinelexer.UpdateStatement:
		return e.executeUpdate(s)
	case *citrinelexer.DeleteStatement:
		return e.executeDelete(s)
	case *citrinelexer.BeginStatement:
		return e.executeBegin(s)
	case *citrinelexer.CommitStatement:
		return e.executeCommit(s)
	case *citrinelexer.RollbackStatement:
		return e.executeRollback(s)
	default:
		return nil, ErrUnsupportedStatement
	}
}

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

		if err := tableInfo.HeapFile.Update(rid, rec); err != nil {
			return nil, fmt.Errorf("executor: update failed: %w", err)
		}

		result.RowsAffected++
	}

	result.Message = fmt.Sprintf("%d row(s) updated", result.RowsAffected)
	return result, nil
}

// executeDelete handles DELETE statements
func (e *Executor) executeDelete(stmt *citrinelexer.DeleteStatement) (*Result, error) {
	tableName := stmt.From.Name
	tableInfo, err := e.catalog.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("executor: table '%s' not found", tableName)
	}

	result := NewResult()
	var toDelete []record.RecordID

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

		toDelete = append(toDelete, rid)
	}

	for _, rid := range toDelete {
		if err := tableInfo.HeapFile.Delete(rid); err != nil {
			return nil, fmt.Errorf("executor: delete failed: %w", err)
		}
		result.RowsAffected++
	}

	result.Message = fmt.Sprintf("%d row(s) deleted", result.RowsAffected)
	return result, nil
}

// evaluateWhere evaluates a WHERE clause expression
func (e *Executor) evaluateWhere(expr citrinelexer.Expression, rec *record.Record, schema *record.Schema) (bool, error) {
	switch ex := expr.(type) {
	case *citrinelexer.BinaryExpression:
		return e.evaluateBinaryExpr(ex, rec, schema)
	case *citrinelexer.BooleanLiteral:
		return ex.Value, nil
	case *citrinelexer.Identifier:
		// Check if it's a column with boolean value
		colIdx, ok := schema.FieldIndex(ex.Name)
		if ok {
			val, err := rec.Get(colIdx)
			if err != nil {
				return false, err
			}
			if val.Type == record.FieldTypeBool {
				b, _ := val.AsBool()
				return b, nil
			}
		}
		return false, nil
	default:
		return false, ErrInvalidExpression
	}
}

// evaluateBinaryExpr evaluates a binary expression
func (e *Executor) evaluateBinaryExpr(expr *citrinelexer.BinaryExpression, rec *record.Record, schema *record.Schema) (bool, error) {
	switch expr.Operator {
	case "AND":
		left, err := e.evaluateWhere(expr.Left, rec, schema)
		if err != nil {
			return false, err
		}
		if !left {
			return false, nil
		}
		return e.evaluateWhere(expr.Right, rec, schema)
	case "OR":
		left, err := e.evaluateWhere(expr.Left, rec, schema)
		if err != nil {
			return false, err
		}
		if left {
			return true, nil
		}
		return e.evaluateWhere(expr.Right, rec, schema)
	}

	var leftVal interface{}
	if ident, ok := expr.Left.(*citrinelexer.Identifier); ok {
		colIdx, ok := schema.FieldIndex(ident.Name)
		if !ok {
			return false, fmt.Errorf("executor: column '%s' not found", ident.Name)
		}
		val, err := rec.Get(colIdx)
		if err != nil {
			return false, err
		}
		leftVal = valueToInterface(val)
	} else {
		leftVal = e.expressionToInterface(expr.Left)
	}

	rightVal := e.expressionToInterface(expr.Right)

	return e.compare(leftVal, rightVal, expr.Operator)
}

// compare compares two values with the given operator
func (e *Executor) compare(left, right interface{}, op string) (bool, error) {
	switch op {
	case "=", "==":
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right), nil
	case "!=", "<>":
		return fmt.Sprintf("%v", left) != fmt.Sprintf("%v", right), nil
	case "<":
		return compareNumeric(left, right) < 0, nil
	case "<=":
		return compareNumeric(left, right) <= 0, nil
	case ">":
		return compareNumeric(left, right) > 0, nil
	case ">=":
		return compareNumeric(left, right) >= 0, nil
	default:
		return false, fmt.Errorf("executor: unsupported operator '%s'", op)
	}
}

func compareNumeric(left, right interface{}) int {
	leftNum := toFloat64(left)
	rightNum := toFloat64(right)

	if leftNum < rightNum {
		return -1
	}
	if leftNum > rightNum {
		return 1
	}
	return 0
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int8:
		return float64(val)
	case int16:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case float32:
		return float64(val)
	case float64:
		return val
	default:
		return 0
	}
}

func (e *Executor) expressionToInterface(expr citrinelexer.Expression) interface{} {
	switch ex := expr.(type) {
	case *citrinelexer.NumberLiteral:
		// Try to parse as int first, then float
		if i, err := citrinelexer.ParseInt(ex.Value); err == nil {
			return i
		}
		if f, err := citrinelexer.ParseFloat(ex.Value); err == nil {
			return f
		}
		return ex.Value
	case *citrinelexer.StringLiteral:
		return ex.Value
	case *citrinelexer.BooleanLiteral:
		return ex.Value
	case *citrinelexer.Identifier:
		return ex.Name
	default:
		return nil
	}
}

func (e *Executor) expressionToValue(expr citrinelexer.Expression, targetType record.FieldType) (record.Value, error) {
	switch ex := expr.(type) {
	case *citrinelexer.NumberLiteral:
		num, err := citrinelexer.ParseFloat(ex.Value)
		if err != nil {
			return record.Value{}, fmt.Errorf("invalid number: %s", ex.Value)
		}
		return numberToValue(num, targetType)
	case *citrinelexer.StringLiteral:
		if targetType == record.FieldTypeString {
			return record.StringValue(ex.Value), nil
		}
		return record.Value{}, ErrTypeMismatch
	case *citrinelexer.BooleanLiteral:
		if targetType == record.FieldTypeBool {
			return record.BoolValue(ex.Value), nil
		}
		return record.Value{}, ErrTypeMismatch
	case *citrinelexer.NullLiteral:
		return record.NullValue(), nil
	case *citrinelexer.Identifier:
		if ex.Name == "NULL" {
			return record.NullValue(), nil
		}
		return record.Value{}, ErrInvalidExpression
	default:
		return record.Value{}, ErrInvalidExpression
	}
}

// evaluateExpressionWithContext evaluates an expression with row context (for UPDATE SET)
func (e *Executor) evaluateExpressionWithContext(expr citrinelexer.Expression, targetType record.FieldType, rec *record.Record, schema *record.Schema) (record.Value, error) {
	switch ex := expr.(type) {
	case *citrinelexer.NumberLiteral:
		num, err := citrinelexer.ParseFloat(ex.Value)
		if err != nil {
			return record.Value{}, fmt.Errorf("invalid number: %s", ex.Value)
		}
		return numberToValue(num, targetType)
	case *citrinelexer.StringLiteral:
		if targetType == record.FieldTypeString {
			return record.StringValue(ex.Value), nil
		}
		return record.Value{}, ErrTypeMismatch
	case *citrinelexer.BooleanLiteral:
		if targetType == record.FieldTypeBool {
			return record.BoolValue(ex.Value), nil
		}
		return record.Value{}, ErrTypeMismatch
	case *citrinelexer.NullLiteral:
		return record.NullValue(), nil
	case *citrinelexer.Identifier:
		if ex.Name == "NULL" {
			return record.NullValue(), nil
		}
		// Column reference - get value from current row
		colIdx, ok := schema.FieldIndex(ex.Name)
		if !ok {
			return record.Value{}, fmt.Errorf("column '%s' not found", ex.Name)
		}
		val, err := rec.Get(colIdx)
		if err != nil {
			return record.Value{}, err
		}
		return val, nil
	case *citrinelexer.BinaryExpression:
		return e.evaluateBinaryExpressionWithContext(ex, targetType, rec, schema)
	default:
		return record.Value{}, ErrInvalidExpression
	}
}

// evaluateBinaryExpressionWithContext evaluates arithmetic expressions like budget - 50000
func (e *Executor) evaluateBinaryExpressionWithContext(expr *citrinelexer.BinaryExpression, targetType record.FieldType, rec *record.Record, schema *record.Schema) (record.Value, error) {
	leftVal, err := e.evaluateExpressionWithContext(expr.Left, targetType, rec, schema)
	if err != nil {
		return record.Value{}, err
	}

	rightVal, err := e.evaluateExpressionWithContext(expr.Right, targetType, rec, schema)
	if err != nil {
		return record.Value{}, err
	}

	// Convert to float64 for arithmetic
	leftNum, err := valueToFloat64(leftVal)
	if err != nil {
		return record.Value{}, fmt.Errorf("left operand: %w", err)
	}

	rightNum, err := valueToFloat64(rightVal)
	if err != nil {
		return record.Value{}, fmt.Errorf("right operand: %w", err)
	}

	var result float64
	switch expr.Operator {
	case "+":
		result = leftNum + rightNum
	case "-":
		result = leftNum - rightNum
	case "*":
		result = leftNum * rightNum
	case "/":
		if rightNum == 0 {
			return record.Value{}, fmt.Errorf("division by zero")
		}
		result = leftNum / rightNum
	case "%":
		if rightNum == 0 {
			return record.Value{}, fmt.Errorf("modulo by zero")
		}
		result = float64(int64(leftNum) % int64(rightNum))
	default:
		return record.Value{}, fmt.Errorf("unsupported operator: %s", expr.Operator)
	}

	return numberToValue(result, targetType)
}

// valueToFloat64 converts a record.Value to float64 for arithmetic
func valueToFloat64(v record.Value) (float64, error) {
	if v.IsNull {
		return 0, fmt.Errorf("cannot perform arithmetic on NULL")
	}

	switch v.Type {
	case record.FieldTypeInt8:
		val, _ := v.AsInt8()
		return float64(val), nil
	case record.FieldTypeInt16:
		val, _ := v.AsInt16()
		return float64(val), nil
	case record.FieldTypeInt32:
		val, _ := v.AsInt32()
		return float64(val), nil
	case record.FieldTypeInt64:
		val, _ := v.AsInt64()
		return float64(val), nil
	case record.FieldTypeFloat32:
		val, _ := v.AsFloat32()
		return float64(val), nil
	case record.FieldTypeFloat64:
		val, _ := v.AsFloat64()
		return val, nil
	default:
		return 0, fmt.Errorf("cannot convert %v to number", v.Type)
	}
}

func numberToValue(num float64, targetType record.FieldType) (record.Value, error) {
	switch targetType {
	case record.FieldTypeInt8:
		return record.Int8Value(int8(num)), nil
	case record.FieldTypeInt16:
		return record.Int16Value(int16(num)), nil
	case record.FieldTypeInt32:
		return record.Int32Value(int32(num)), nil
	case record.FieldTypeInt64:
		return record.Int64Value(int64(num)), nil
	case record.FieldTypeFloat32:
		return record.Float32Value(float32(num)), nil
	case record.FieldTypeFloat64:
		return record.Float64Value(num), nil
	default:
		return record.Value{}, ErrTypeMismatch
	}
}

func valueToInterface(v record.Value) interface{} {
	if v.IsNull {
		return nil
	}

	switch v.Type {
	case record.FieldTypeInt8:
		val, _ := v.AsInt8()
		return val
	case record.FieldTypeInt16:
		val, _ := v.AsInt16()
		return val
	case record.FieldTypeInt32:
		val, _ := v.AsInt32()
		return val
	case record.FieldTypeInt64:
		val, _ := v.AsInt64()
		return val
	case record.FieldTypeFloat32:
		val, _ := v.AsFloat32()
		return val
	case record.FieldTypeFloat64:
		val, _ := v.AsFloat64()
		return val
	case record.FieldTypeBool:
		val, _ := v.AsBool()
		return val
	case record.FieldTypeString:
		val, _ := v.AsString()
		return val
	case record.FieldTypeBytes:
		val, _ := v.AsBytes()
		return val
	default:
		return nil
	}
}

func mapDataType(dt string) (record.FieldType, error) {
	switch dt {
	case "INTEGER", "INT":
		return record.FieldTypeInt32, nil
	case "BIGINT":
		return record.FieldTypeInt64, nil
	case "SMALLINT":
		return record.FieldTypeInt16, nil
	case "TINYINT":
		return record.FieldTypeInt8, nil
	case "REAL", "FLOAT", "DOUBLE":
		return record.FieldTypeFloat64, nil
	case "TEXT", "VARCHAR", "CHAR", "STRING":
		return record.FieldTypeString, nil
	case "BLOB", "BINARY":
		return record.FieldTypeBytes, nil
	case "BOOLEAN", "BOOL":
		return record.FieldTypeBool, nil
	default:
		return 0, fmt.Errorf("%w: %s", ErrInvalidDataType, dt)
	}
}

func (e *Executor) buildIndexKey(rec *record.Record, columns []string, schema *record.Schema) string {
	key := ""
	for i, col := range columns {
		colIdx, _ := schema.FieldIndex(col)
		val, _ := rec.Get(colIdx)
		if i > 0 {
			key += "|"
		}
		key += fmt.Sprintf("%v", valueToInterface(val))
	}
	return key
}

// executeBegin handles BEGIN/BEGIN TRANSACTION statements
func (e *Executor) executeBegin(_ *citrinelexer.BeginStatement) (*Result, error) {
	if e.inTx {
		return nil, ErrTransactionActive
	}

	result := NewResult()

	if e.wal != nil {
		txID, err := e.wal.BeginTransaction(30 * time.Second)
		if err != nil {
			return nil, fmt.Errorf("executor: failed to begin transaction: %w", err)
		}
		e.activeTxID = txID
		e.inTx = true
		result.Message = fmt.Sprintf("Transaction started (ID: %s)", txID)
	} else {
		e.inTx = true
		result.Message = "Transaction started (WAL not configured)"
	}

	return result, nil
}

// executeCommit handles COMMIT statements
func (e *Executor) executeCommit(_ *citrinelexer.CommitStatement) (*Result, error) {
	if !e.inTx {
		return nil, ErrNoActiveTransaction
	}

	result := NewResult()

	if e.wal != nil && e.activeTxID != "" {
		indexes, err := e.wal.CommitTransaction(e.activeTxID)
		if err != nil {
			return nil, fmt.Errorf("executor: failed to commit transaction: %w", err)
		}
		result.Message = fmt.Sprintf("Transaction committed (%d entries written)", len(indexes))
		e.activeTxID = ""
	} else {
		result.Message = "Transaction committed"
	}

	e.inTx = false
	return result, nil
}

// executeRollback handles ROLLBACK statements
func (e *Executor) executeRollback(_ *citrinelexer.RollbackStatement) (*Result, error) {
	if !e.inTx {
		return nil, ErrNoActiveTransaction
	}

	result := NewResult()

	if e.wal != nil && e.activeTxID != "" {
		if err := e.wal.RollbackTransaction(e.activeTxID); err != nil {
			return nil, fmt.Errorf("executor: failed to rollback transaction: %w", err)
		}
		result.Message = "Transaction rolled back"
		e.activeTxID = ""
	} else {
		result.Message = "Transaction rolled back"
	}

	e.inTx = false
	return result, nil
}

// InTransaction returns whether a transaction is active
func (e *Executor) InTransaction() bool {
	return e.inTx
}

// logToWAL logs an operation to the WAL if in transaction
func (e *Executor) logToWAL(operation string, data []byte) error {
	if e.wal == nil || !e.inTx {
		return nil
	}

	logEntry := fmt.Sprintf("%s:%s", operation, string(data))
	return e.wal.AddToTransaction(e.activeTxID, []byte(logEntry))
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
	type joinedRow struct {
		records map[string]*record.Record // alias -> record
	}

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

		for _, jr := range joinedRows {
			matched := false
			for _, rightRec := range joinTableRows {
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
			}

			// For LEFT JOIN, include unmatched left rows
			if !matched && ji.joinType == "LEFT" {
				newJR := &joinedRow{records: make(map[string]*record.Record)}
				for k, v := range jr.records {
					newJR.records[k] = v
				}
				newJR.records[ji.alias] = nil // NULL for right side
				newJoinedRows = append(newJoinedRows, newJR)
			}
		}

		joinedRows = newJoinedRows
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

	return result, nil
}

// tableContext holds table info for JOIN operations
type tableContext struct {
	tableName    string
	tableInfo    *catalog.TableInfo
	schemaOffset int
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
