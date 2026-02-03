package planner

import (
	"testing"

	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

func TestNewPlanner(t *testing.T) {
	p := NewPlanner(nil)
	if p == nil {
		t.Fatal("expected planner")
	}
}

func TestPlanSelectNoFrom(t *testing.T) {
	p := NewPlanner(nil)
	stmt := &citrinelexer.SelectStatement{}
	_, err := p.PlanSelect(stmt)
	if err != ErrNoFromClause {
		t.Errorf("expected ErrNoFromClause, got %v", err)
	}
}

func TestScanTypeString(t *testing.T) {
	if FullTableScan.String() != "FullTableScan" {
		t.Error("wrong string for FullTableScan")
	}
	if IndexScan.String() != "IndexScan" {
		t.Error("wrong string for IndexScan")
	}
}

func TestPlanInterfaces(t *testing.T) {
	scan := &ScanPlan{TableName: "t", ScanType: FullTableScan, EstimatedRows: 100, EstimatedCost: 100}
	if scan.Type() != "FullTableScan" {
		t.Error("wrong type")
	}
	if scan.Cost() != 100 {
		t.Error("wrong cost")
	}

	filter := &FilterPlan{Input: scan}
	if filter.Type() != "Filter" {
		t.Error("wrong type")
	}

	proj := &ProjectPlan{Input: scan, Columns: []string{"a"}}
	if proj.Type() != "Project" {
		t.Error("wrong type")
	}

	sort := &SortPlan{Input: scan, EstimatedCost: 200}
	if sort.Type() != "Sort" {
		t.Error("wrong type")
	}

	limit := &LimitPlan{Input: scan, Limit: 10}
	if limit.Type() != "Limit" {
		t.Error("wrong type")
	}
}

func TestUpdateStats(t *testing.T) {
	p := NewPlanner(nil)
	p.UpdateStats("t", &TableStats{RowCount: 500})
	if p.estimateRowCount("t") != 500 {
		t.Error("wrong row count")
	}
	if p.estimateRowCount("x") != 1000 {
		t.Error("expected default 1000")
	}
}

func TestExplainPlan(t *testing.T) {
	scan := &ScanPlan{TableName: "t", ScanType: FullTableScan, EstimatedRows: 10, EstimatedCost: 10}
	proj := &ProjectPlan{Input: scan, Columns: []string{"a"}}
	result := ExplainPlan(proj, 0)
	if len(result) < 5 {
		t.Error("expected explain output")
	}
}

func TestExtractColumns(t *testing.T) {
	p := NewPlanner(nil)
	schema := record.NewSchema([]record.Field{
		{Name: "a", Type: record.FieldTypeInt32},
		{Name: "b", Type: record.FieldTypeString},
	})
	fields := []citrinelexer.Expression{&citrinelexer.Identifier{Name: "*"}}
	cols := p.extractColumns(fields, schema)
	if len(cols) != 2 {
		t.Errorf("expected 2, got %d", len(cols))
	}
}

func TestFindBestIndex(t *testing.T) {
	p := NewPlanner(nil)
	ti := &catalog.TableInfo{
		Name: "t",
		Indexes: map[string]*catalog.IndexInfo{
			"idx": {Name: "idx", Columns: []string{"id"}},
		},
	}
	expr := &citrinelexer.BinaryExpression{
		Left:  &citrinelexer.Identifier{Name: "id"},
		Right: &citrinelexer.NumberLiteral{Value: "1"},
	}
	idx := p.findBestIndex(ti, expr)
	if idx == nil || idx.Name != "idx" {
		t.Error("expected to find index")
	}
}
