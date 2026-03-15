package executor

import (
	"fmt"

	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

// toFloat64 converts an interface value to float64
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

// valueToInterface converts a record.Value to a Go interface{}
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

// numberToValue converts a float64 to a record.Value of the given type
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

// mapDataType maps SQL type strings to record field types
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

// expressionToInterface converts a parsed expression to a Go value
func (e *Executor) expressionToInterface(expr citrinelexer.Expression) interface{} {
	switch ex := expr.(type) {
	case *citrinelexer.NumberLiteral:
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

// expressionToValue converts a parsed expression to a record.Value
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

// buildIndexKey builds a string key from record values for index operations
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
