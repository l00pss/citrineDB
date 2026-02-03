package executor

import "testing"

func TestAggregateTypeString(t *testing.T) {
	if AggCount.String() != "COUNT" {
		t.Error("wrong string")
	}
	if AggSum.String() != "SUM" {
		t.Error("wrong string")
	}
	if AggAvg.String() != "AVG" {
		t.Error("wrong string")
	}
	if AggMin.String() != "MIN" {
		t.Error("wrong string")
	}
	if AggMax.String() != "MAX" {
		t.Error("wrong string")
	}
}

func TestCountWithoutGroup(t *testing.T) {
	ae := NewAggregateExecutor()
	ae.SetData([]string{"id", "name"}, [][]interface{}{
		{1, "A"},
		{2, "B"},
		{3, "C"},
	})
	ae.AddAggregate(AggCount, "*", "cnt")

	cols, rows := ae.Execute()

	if len(cols) != 1 || cols[0] != "cnt" {
		t.Error("wrong columns")
	}
	if len(rows) != 1 {
		t.Error("expected 1 row")
	}
	if rows[0][0].(int64) != 3 {
		t.Errorf("expected 3, got %v", rows[0][0])
	}
}

func TestSumAvg(t *testing.T) {
	ae := NewAggregateExecutor()
	ae.SetData([]string{"val"}, [][]interface{}{
		{10},
		{20},
		{30},
	})
	ae.AddAggregate(AggSum, "val", "total")
	ae.AddAggregate(AggAvg, "val", "average")

	cols, rows := ae.Execute()

	if len(cols) != 2 {
		t.Errorf("expected 2 columns, got %d", len(cols))
	}
	if rows[0][0].(float64) != 60 {
		t.Errorf("sum: expected 60, got %v", rows[0][0])
	}
	if rows[0][1].(float64) != 20 {
		t.Errorf("avg: expected 20, got %v", rows[0][1])
	}
}

func TestMinMax(t *testing.T) {
	ae := NewAggregateExecutor()
	ae.SetData([]string{"val"}, [][]interface{}{
		{5},
		{2},
		{8},
		{1},
	})
	ae.AddAggregate(AggMin, "val", "min_val")
	ae.AddAggregate(AggMax, "val", "max_val")

	_, rows := ae.Execute()

	if toFloat(rows[0][0]) != 1 {
		t.Errorf("min: expected 1, got %v", rows[0][0])
	}
	if toFloat(rows[0][1]) != 8 {
		t.Errorf("max: expected 8, got %v", rows[0][1])
	}
}

func TestGroupBy(t *testing.T) {
	ae := NewAggregateExecutor()
	ae.SetData([]string{"dept", "salary"}, [][]interface{}{
		{"IT", 100},
		{"IT", 150},
		{"HR", 80},
		{"HR", 90},
		{"HR", 100},
	})
	ae.SetGroupBy([]string{"dept"})
	ae.AddAggregate(AggCount, "*", "cnt")
	ae.AddAggregate(AggSum, "salary", "total")

	cols, rows := ae.Execute()

	if len(cols) != 3 {
		t.Errorf("expected 3 columns, got %d", len(cols))
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 groups, got %d", len(rows))
	}

	hrRow := findGroupRow(rows, "HR")
	if hrRow == nil {
		t.Fatal("HR group not found")
	}
	if hrRow[1].(int64) != 3 {
		t.Errorf("HR count: expected 3, got %v", hrRow[1])
	}
	if hrRow[2].(float64) != 270 {
		t.Errorf("HR total: expected 270, got %v", hrRow[2])
	}
}

func findGroupRow(rows [][]interface{}, key string) []interface{} {
	for _, row := range rows {
		if row[0] == key {
			return row
		}
	}
	return nil
}

func TestEmptyData(t *testing.T) {
	ae := NewAggregateExecutor()
	ae.SetData([]string{"val"}, [][]interface{}{})
	ae.AddAggregate(AggCount, "*", "cnt")

	_, rows := ae.Execute()

	if len(rows) != 1 {
		t.Error("expected 1 row")
	}
	if rows[0][0].(int64) != 0 {
		t.Errorf("expected 0, got %v", rows[0][0])
	}
}

func TestNullHandling(t *testing.T) {
	ae := NewAggregateExecutor()
	ae.SetData([]string{"val"}, [][]interface{}{
		{10},
		{nil},
		{20},
	})
	ae.AddAggregate(AggCount, "val", "cnt")
	ae.AddAggregate(AggSum, "val", "total")

	_, rows := ae.Execute()

	if rows[0][0].(int64) != 2 {
		t.Errorf("count: expected 2, got %v", rows[0][0])
	}
	if rows[0][1].(float64) != 30 {
		t.Errorf("sum: expected 30, got %v", rows[0][1])
	}
}
