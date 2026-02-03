package executor

import "testing"

func TestSortAsc(t *testing.T) {
	se := NewSortExecutor()
	se.SetData([]string{"name", "age"}, [][]interface{}{
		{"Charlie", 30},
		{"Alice", 25},
		{"Bob", 35},
	})
	se.AddSort("name", false)
	result := se.Execute()

	if result[0][0] != "Alice" {
		t.Errorf("first should be Alice, got %v", result[0][0])
	}
	if result[1][0] != "Bob" {
		t.Errorf("second should be Bob, got %v", result[1][0])
	}
	if result[2][0] != "Charlie" {
		t.Errorf("third should be Charlie, got %v", result[2][0])
	}
}

func TestSortDesc(t *testing.T) {
	se := NewSortExecutor()
	se.SetData([]string{"val"}, [][]interface{}{
		{1},
		{3},
		{2},
	})
	se.AddSort("val", true)
	result := se.Execute()

	if result[0][0] != 3 {
		t.Errorf("first should be 3, got %v", result[0][0])
	}
	if result[2][0] != 1 {
		t.Errorf("last should be 1, got %v", result[2][0])
	}
}

func TestSortMultiColumn(t *testing.T) {
	se := NewSortExecutor()
	se.SetData([]string{"dept", "salary"}, [][]interface{}{
		{"IT", 100},
		{"HR", 80},
		{"IT", 90},
		{"HR", 100},
	})
	se.AddSort("dept", false)
	se.AddSort("salary", true)
	result := se.Execute()

	if result[0][0] != "HR" || result[0][1] != 100 {
		t.Errorf("first should be HR,100, got %v,%v", result[0][0], result[0][1])
	}
}

func TestSortEmpty(t *testing.T) {
	se := NewSortExecutor()
	se.SetData([]string{"val"}, [][]interface{}{})
	se.AddSort("val", false)
	result := se.Execute()

	if len(result) != 0 {
		t.Error("expected empty result")
	}
}

func TestSortNulls(t *testing.T) {
	se := NewSortExecutor()
	se.SetData([]string{"val"}, [][]interface{}{
		{2},
		{nil},
		{1},
	})
	se.AddSort("val", false)
	result := se.Execute()

	if result[0][0] != nil {
		t.Errorf("nil should come first, got %v", result[0][0])
	}
}

func TestLimitBasic(t *testing.T) {
	le := NewLimitExecutor()
	le.SetData([][]interface{}{
		{1}, {2}, {3}, {4}, {5},
	})
	le.SetLimit(3)
	result := le.Execute()

	if len(result) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result))
	}
	if result[0][0] != 1 || result[2][0] != 3 {
		t.Error("wrong values")
	}
}

func TestLimitOffset(t *testing.T) {
	le := NewLimitExecutor()
	le.SetData([][]interface{}{
		{1}, {2}, {3}, {4}, {5},
	})
	le.SetLimit(2)
	le.SetOffset(2)
	result := le.Execute()

	if len(result) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result))
	}
	if result[0][0] != 3 || result[1][0] != 4 {
		t.Error("wrong values")
	}
}

func TestLimitBeyondSize(t *testing.T) {
	le := NewLimitExecutor()
	le.SetData([][]interface{}{
		{1}, {2},
	})
	le.SetLimit(10)
	result := le.Execute()

	if len(result) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result))
	}
}

func TestOffsetBeyondSize(t *testing.T) {
	le := NewLimitExecutor()
	le.SetData([][]interface{}{
		{1}, {2},
	})
	le.SetOffset(10)
	result := le.Execute()

	if len(result) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result))
	}
}

func TestNoLimit(t *testing.T) {
	le := NewLimitExecutor()
	le.SetData([][]interface{}{
		{1}, {2}, {3},
	})
	result := le.Execute()

	if len(result) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result))
	}
}

func TestEmptyLimit(t *testing.T) {
	le := NewLimitExecutor()
	le.SetData([][]interface{}{})
	le.SetLimit(5)
	result := le.Execute()

	if len(result) != 0 {
		t.Error("expected empty result")
	}
}
