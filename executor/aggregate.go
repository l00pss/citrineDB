package executor

import (
	"sort"
)

type AggregateType int

const (
	AggCount AggregateType = iota
	AggSum
	AggAvg
	AggMin
	AggMax
)

func (a AggregateType) String() string {
	switch a {
	case AggCount:
		return "COUNT"
	case AggSum:
		return "SUM"
	case AggAvg:
		return "AVG"
	case AggMin:
		return "MIN"
	case AggMax:
		return "MAX"
	default:
		return "UNKNOWN"
	}
}

type AggregateFunc struct {
	Type   AggregateType
	Column string
	Alias  string
}

type AggregateExecutor struct {
	groupByColumns []string
	aggregates     []AggregateFunc
	columns        []string
	rows           [][]interface{}
}

func NewAggregateExecutor() *AggregateExecutor {
	return &AggregateExecutor{}
}

func (a *AggregateExecutor) SetData(cols []string, rows [][]interface{}) {
	a.columns = cols
	a.rows = rows
}

func (a *AggregateExecutor) SetGroupBy(cols []string) {
	a.groupByColumns = cols
}

func (a *AggregateExecutor) AddAggregate(aggType AggregateType, column, alias string) {
	a.aggregates = append(a.aggregates, AggregateFunc{
		Type:   aggType,
		Column: column,
		Alias:  alias,
	})
}

func (a *AggregateExecutor) Execute() ([]string, [][]interface{}) {
	if len(a.groupByColumns) == 0 {
		return a.executeWithoutGroupBy()
	}
	return a.executeWithGroupBy()
}

func (a *AggregateExecutor) executeWithoutGroupBy() ([]string, [][]interface{}) {
	resultCols := make([]string, len(a.aggregates))
	for i, agg := range a.aggregates {
		if agg.Alias != "" {
			resultCols[i] = agg.Alias
		} else {
			resultCols[i] = agg.Type.String() + "(" + agg.Column + ")"
		}
	}

	resultRow := make([]interface{}, len(a.aggregates))
	for i, agg := range a.aggregates {
		resultRow[i] = a.computeAggregate(agg, a.rows)
	}

	return resultCols, [][]interface{}{resultRow}
}

func (a *AggregateExecutor) executeWithGroupBy() ([]string, [][]interface{}) {
	groups := a.groupRows()

	resultCols := make([]string, 0, len(a.groupByColumns)+len(a.aggregates))
	resultCols = append(resultCols, a.groupByColumns...)
	for _, agg := range a.aggregates {
		if agg.Alias != "" {
			resultCols = append(resultCols, agg.Alias)
		} else {
			resultCols = append(resultCols, agg.Type.String()+"("+agg.Column+")")
		}
	}

	var results [][]interface{}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		groupRows := groups[key]
		row := make([]interface{}, 0, len(a.groupByColumns)+len(a.aggregates))

		for _, col := range a.groupByColumns {
			idx := a.findColIndex(col)
			if idx >= 0 && len(groupRows) > 0 {
				row = append(row, groupRows[0][idx])
			} else {
				row = append(row, nil)
			}
		}

		for _, agg := range a.aggregates {
			row = append(row, a.computeAggregate(agg, groupRows))
		}

		results = append(results, row)
	}

	return resultCols, results
}

func (a *AggregateExecutor) groupRows() map[string][][]interface{} {
	groups := make(map[string][][]interface{})

	for _, row := range a.rows {
		key := a.makeGroupKey(row)
		groups[key] = append(groups[key], row)
	}

	return groups
}

func (a *AggregateExecutor) makeGroupKey(row []interface{}) string {
	key := ""
	for i, col := range a.groupByColumns {
		if i > 0 {
			key += "|"
		}
		idx := a.findColIndex(col)
		if idx >= 0 {
			key += toStr(row[idx])
		}
	}
	return key
}

func (a *AggregateExecutor) computeAggregate(agg AggregateFunc, rows [][]interface{}) interface{} {
	colIdx := a.findColIndex(agg.Column)

	switch agg.Type {
	case AggCount:
		if agg.Column == "*" {
			return int64(len(rows))
		}
		count := int64(0)
		for _, row := range rows {
			if colIdx >= 0 && row[colIdx] != nil {
				count++
			}
		}
		return count

	case AggSum:
		if colIdx < 0 {
			return nil
		}
		sum := float64(0)
		for _, row := range rows {
			sum += toFloat(row[colIdx])
		}
		return sum

	case AggAvg:
		if colIdx < 0 {
			return nil
		}
		sum := float64(0)
		count := 0
		for _, row := range rows {
			if row[colIdx] != nil {
				sum += toFloat(row[colIdx])
				count++
			}
		}
		if count == 0 {
			return nil
		}
		return sum / float64(count)

	case AggMin:
		if colIdx < 0 {
			return nil
		}
		var min interface{}
		for _, row := range rows {
			v := row[colIdx]
			if v == nil {
				continue
			}
			if min == nil || compareValues(v, min) < 0 {
				min = v
			}
		}
		return min

	case AggMax:
		if colIdx < 0 {
			return nil
		}
		var max interface{}
		for _, row := range rows {
			v := row[colIdx]
			if v == nil {
				continue
			}
			if max == nil || compareValues(v, max) > 0 {
				max = v
			}
		}
		return max

	default:
		return nil
	}
}

func (a *AggregateExecutor) findColIndex(name string) int {
	for i, c := range a.columns {
		if c == name {
			return i
		}
	}
	return -1
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
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

func compareValues(a, b interface{}) int {
	af := toFloat(a)
	bf := toFloat(b)
	if af < bf {
		return -1
	}
	if af > bf {
		return 1
	}
	return 0
}
