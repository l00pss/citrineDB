package executor

import "testing"

func TestJoinTypeString(t *testing.T) {
	if InnerJoin.String() != "INNER JOIN" {
		t.Error("wrong string")
	}
	if LeftJoin.String() != "LEFT JOIN" {
		t.Error("wrong string")
	}
}

func TestInnerJoin(t *testing.T) {
	je := NewJoinExecutor(InnerJoin)
	je.SetLeftTable("u", []string{"id", "name"}, [][]interface{}{{1, "A"}, {2, "B"}})
	je.SetRightTable("o", []string{"uid", "prod"}, [][]interface{}{{1, "X"}, {1, "Y"}})
	je.SetOnCondition("id", "uid")
	cols, rows := je.Execute()
	if len(cols) != 4 {
		t.Errorf("expected 4 cols, got %d", len(cols))
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
}

func TestLeftJoin(t *testing.T) {
	je := NewJoinExecutor(LeftJoin)
	je.SetLeftTable("u", []string{"id"}, [][]interface{}{{1}, {2}})
	je.SetRightTable("o", []string{"uid"}, [][]interface{}{{1}})
	je.SetOnCondition("id", "uid")
	_, rows := je.Execute()
	if len(rows) != 2 {
		t.Errorf("expected 2, got %d", len(rows))
	}
}

func TestRightJoin(t *testing.T) {
	je := NewJoinExecutor(RightJoin)
	je.SetLeftTable("u", []string{"id"}, [][]interface{}{{1}})
	je.SetRightTable("o", []string{"uid"}, [][]interface{}{{1}, {2}})
	je.SetOnCondition("id", "uid")
	_, rows := je.Execute()
	if len(rows) != 2 {
		t.Errorf("expected 2, got %d", len(rows))
	}
}

func TestCrossJoin(t *testing.T) {
	je := NewJoinExecutor(CrossJoin)
	je.SetLeftTable("a", []string{"x"}, [][]interface{}{{1}, {2}})
	je.SetRightTable("b", []string{"y"}, [][]interface{}{{"a"}, {"b"}})
	_, rows := je.Execute()
	if len(rows) != 4 {
		t.Errorf("expected 4, got %d", len(rows))
	}
}

func TestResultColumns(t *testing.T) {
	je := NewJoinExecutor(InnerJoin)
	je.SetLeftTable("u", []string{"id"}, nil)
	je.SetRightTable("o", []string{"uid"}, nil)
	cols := je.ResultColumns()
	if cols[0] != "u.id" || cols[1] != "o.uid" {
		t.Error("wrong column names")
	}
}
