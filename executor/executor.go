package executor

import (
	"errors"
	"fmt"

	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

var (
	ErrUnsupportedStatement = errors.New("executor: unsupported statement type")
	ErrInvalidExpression    = errors.New("executor: invalid expression")
	ErrTypeMismatch         = errors.New("executor: type mismatch")
	ErrNullConstraint       = errors.New("executor: null constraint violation")
	ErrInvalidDataType      = errors.New("executor: invalid data type")
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
	catalog *catalog.Catalog
}

// NewExecutor creates a new executor
func NewExecutor(cat *catalog.Catalog) *Executor {
	return &Executor{
		catalog: cat,
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
	if stmt.From == nil || stmt.From.Name == nil {
		return nil, errors.New("executor: SELECT requires FROM clause")
	}

	tableName := stmt.From.Name.Name
	tableInfo, err := e.catalog.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("executor: table '%s' not found", tableName)
	}

	selectAll := false
	var columnNames []string

	for _, field := range stmt.Fields {
		if ident, ok := field.(*citrinelexer.Identifier); ok {
			if ident.Name == "*" {
				selectAll = true
				break
			}
			columnNames = append(columnNames, ident.Name)
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
			val, err := e.expressionToValue(expr, field.Type)
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
	case *citrinelexer.Identifier:
		if ex.Name == "NULL" {
			return record.NullValue(), nil
		}
		return record.Value{}, ErrInvalidExpression
	default:
		return record.Value{}, ErrInvalidExpression
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
