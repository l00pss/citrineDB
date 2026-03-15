package executor

import (
	"fmt"

	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

// evaluateWhere evaluates a WHERE clause expression
func (e *Executor) evaluateWhere(expr citrinelexer.Expression, rec *record.Record, schema *record.Schema) (bool, error) {
	switch ex := expr.(type) {
	case *citrinelexer.BinaryExpression:
		return e.evaluateBinaryExpr(ex, rec, schema)
	case *citrinelexer.BooleanLiteral:
		return ex.Value, nil
	case *citrinelexer.Identifier:
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
	case *citrinelexer.BetweenExpression:
		return e.evaluateBetween(ex, rec, schema)
	case *citrinelexer.InExpression:
		return e.evaluateIn(ex, rec, schema)
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
	case "LIKE":
		return matchLike(fmt.Sprintf("%v", left), fmt.Sprintf("%v", right)), nil
	case "NOT LIKE":
		return !matchLike(fmt.Sprintf("%v", left), fmt.Sprintf("%v", right)), nil
	case "GLOB":
		return matchGlob(fmt.Sprintf("%v", left), fmt.Sprintf("%v", right)), nil
	case "NOT GLOB":
		return !matchGlob(fmt.Sprintf("%v", left), fmt.Sprintf("%v", right)), nil
	default:
		return false, fmt.Errorf("executor: unsupported operator '%s'", op)
	}
}

// compareNumeric compares two values numerically
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

// evaluateBetween evaluates a BETWEEN expression
func (e *Executor) evaluateBetween(expr *citrinelexer.BetweenExpression, rec *record.Record, schema *record.Schema) (bool, error) {
	// Get the value to check
	var val interface{}
	if ident, ok := expr.Expr.(*citrinelexer.Identifier); ok {
		colIdx, ok := schema.FieldIndex(ident.Name)
		if !ok {
			return false, fmt.Errorf("executor: column '%s' not found", ident.Name)
		}
		v, err := rec.Get(colIdx)
		if err != nil {
			return false, err
		}
		val = valueToInterface(v)
	} else {
		val = e.expressionToInterface(expr.Expr)
	}

	low := e.expressionToInterface(expr.Low)
	high := e.expressionToInterface(expr.High)

	// Compare: val >= low AND val <= high
	result := compareNumeric(val, low) >= 0 && compareNumeric(val, high) <= 0

	if expr.Not {
		return !result, nil
	}
	return result, nil
}

// evaluateIn evaluates an IN expression
func (e *Executor) evaluateIn(expr *citrinelexer.InExpression, rec *record.Record, schema *record.Schema) (bool, error) {
	// Get the value to check
	var val interface{}
	if ident, ok := expr.Expr.(*citrinelexer.Identifier); ok {
		colIdx, ok := schema.FieldIndex(ident.Name)
		if !ok {
			return false, fmt.Errorf("executor: column '%s' not found", ident.Name)
		}
		v, err := rec.Get(colIdx)
		if err != nil {
			return false, err
		}
		val = valueToInterface(v)
	} else {
		val = e.expressionToInterface(expr.Expr)
	}

	valStr := fmt.Sprintf("%v", val)

	// Check if value is in the list
	for _, listItem := range expr.Values {
		itemVal := e.expressionToInterface(listItem)
		itemStr := fmt.Sprintf("%v", itemVal)
		if valStr == itemStr {
			if expr.Not {
				return false, nil
			}
			return true, nil
		}
	}

	if expr.Not {
		return true, nil
	}
	return false, nil
}
