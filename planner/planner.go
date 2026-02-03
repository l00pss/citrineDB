package planner

import (
	"fmt"

	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinelexer"
)

type ScanType int

const (
	FullTableScan ScanType = iota
	IndexScan
	IndexOnlyScan
)

func (s ScanType) String() string {
	switch s {
	case FullTableScan:
		return "FullTableScan"
	case IndexScan:
		return "IndexScan"
	case IndexOnlyScan:
		return "IndexOnlyScan"
	default:
		return "Unknown"
	}
}

type Plan interface {
	Type() string
	Cost() float64
	Children() []Plan
}

type ScanPlan struct {
	TableName     string
	ScanType      ScanType
	IndexName     string
	Columns       []string
	Predicate     citrinelexer.Expression
	EstimatedRows int64
	EstimatedCost float64
}

func (p *ScanPlan) Type() string     { return p.ScanType.String() }
func (p *ScanPlan) Cost() float64    { return p.EstimatedCost }
func (p *ScanPlan) Children() []Plan { return nil }

type FilterPlan struct {
	Input                Plan
	Predicate            citrinelexer.Expression
	EstimatedSelectivity float64
}

func (p *FilterPlan) Type() string     { return "Filter" }
func (p *FilterPlan) Cost() float64    { return p.Input.Cost() * 1.1 }
func (p *FilterPlan) Children() []Plan { return []Plan{p.Input} }

type ProjectPlan struct {
	Input   Plan
	Columns []string
}

func (p *ProjectPlan) Type() string     { return "Project" }
func (p *ProjectPlan) Cost() float64    { return p.Input.Cost() * 1.05 }
func (p *ProjectPlan) Children() []Plan { return []Plan{p.Input} }

type SortPlan struct {
	Input         Plan
	OrderBy       []OrderBySpec
	EstimatedCost float64
}

type OrderBySpec struct {
	Column string
	Desc   bool
}

func (p *SortPlan) Type() string     { return "Sort" }
func (p *SortPlan) Cost() float64    { return p.EstimatedCost }
func (p *SortPlan) Children() []Plan { return []Plan{p.Input} }

type LimitPlan struct {
	Input  Plan
	Limit  int64
	Offset int64
}

func (p *LimitPlan) Type() string     { return "Limit" }
func (p *LimitPlan) Cost() float64    { return p.Input.Cost() * 0.5 }
func (p *LimitPlan) Children() []Plan { return []Plan{p.Input} }

type Planner struct {
	catalog *catalog.Catalog
	stats   map[string]*TableStats
}

type TableStats struct {
	RowCount    int64
	AvgRowSize  int
	ColumnStats map[string]*ColumnStats
}

type ColumnStats struct {
	DistinctCount int64
	NullCount     int64
	MinValue      interface{}
	MaxValue      interface{}
}

func NewPlanner(cat *catalog.Catalog) *Planner {
	return &Planner{
		catalog: cat,
		stats:   make(map[string]*TableStats),
	}
}

func (p *Planner) PlanSelect(stmt *citrinelexer.SelectStatement) (Plan, error) {
	if stmt.From == nil || stmt.From.Name == nil {
		return nil, ErrNoFromClause
	}

	tableName := stmt.From.Name.Name
	tableInfo, err := p.catalog.GetTable(tableName)
	if err != nil {
		return nil, err
	}

	columns := p.extractColumns(stmt.Fields, tableInfo.Schema)
	var plan Plan
	plan = p.createScanPlan(tableName, tableInfo, stmt.Where)

	if stmt.Where != nil {
		scanPlan := plan.(*ScanPlan)
		if scanPlan.ScanType == FullTableScan {
			plan = &FilterPlan{
				Input:                plan,
				Predicate:            stmt.Where,
				EstimatedSelectivity: 0.3,
			}
		}
	}

	plan = &ProjectPlan{Input: plan, Columns: columns}

	if len(stmt.OrderBy) > 0 {
		orderSpecs := make([]OrderBySpec, len(stmt.OrderBy))
		for i, ob := range stmt.OrderBy {
			colName := ""
			if ident, ok := ob.Expression.(*citrinelexer.Identifier); ok {
				colName = ident.Name
			}
			orderSpecs[i] = OrderBySpec{Column: colName, Desc: ob.Direction == "DESC"}
		}
		plan = &SortPlan{Input: plan, OrderBy: orderSpecs, EstimatedCost: plan.Cost() * 2}
	}

	if stmt.Limit != nil {
		limit := int64(0)
		offset := int64(0)
		if numLit, ok := stmt.Limit.Count.(*citrinelexer.NumberLiteral); ok {
			l, _ := citrinelexer.ParseInt(numLit.Value)
			limit = int64(l)
		}
		if stmt.Limit.Offset != nil {
			if numLit, ok := stmt.Limit.Offset.(*citrinelexer.NumberLiteral); ok {
				o, _ := citrinelexer.ParseInt(numLit.Value)
				offset = int64(o)
			}
		}
		plan = &LimitPlan{Input: plan, Limit: limit, Offset: offset}
	}

	return plan, nil
}

func (p *Planner) createScanPlan(tableName string, tableInfo *catalog.TableInfo, where citrinelexer.Expression) *ScanPlan {
	plan := &ScanPlan{
		TableName:     tableName,
		ScanType:      FullTableScan,
		EstimatedRows: p.estimateRowCount(tableName),
		EstimatedCost: float64(p.estimateRowCount(tableName)),
	}

	if where == nil {
		return plan
	}

	indexCandidate := p.findBestIndex(tableInfo, where)
	if indexCandidate != nil {
		plan.ScanType = IndexScan
		plan.IndexName = indexCandidate.Name
		plan.Predicate = where
		plan.EstimatedRows = plan.EstimatedRows / 10
		plan.EstimatedCost = float64(plan.EstimatedRows) * 1.5
	}

	return plan
}

func (p *Planner) findBestIndex(tableInfo *catalog.TableInfo, where citrinelexer.Expression) *catalog.IndexInfo {
	if where == nil {
		return nil
	}

	columns := p.extractPredicateColumns(where)
	for _, idx := range tableInfo.Indexes {
		for _, col := range columns {
			if len(idx.Columns) > 0 && idx.Columns[0] == col {
				return idx
			}
		}
	}
	return nil
}

func (p *Planner) extractPredicateColumns(expr citrinelexer.Expression) []string {
	var columns []string

	switch e := expr.(type) {
	case *citrinelexer.BinaryExpression:
		if ident, ok := e.Left.(*citrinelexer.Identifier); ok {
			columns = append(columns, ident.Name)
		}
		if ident, ok := e.Right.(*citrinelexer.Identifier); ok {
			columns = append(columns, ident.Name)
		}
	case *citrinelexer.Identifier:
		columns = append(columns, e.Name)
	}
	return columns
}

func (p *Planner) extractColumns(fields []citrinelexer.Expression, schema *record.Schema) []string {
	var columns []string
	for _, field := range fields {
		if ident, ok := field.(*citrinelexer.Identifier); ok {
			if ident.Name == "*" {
				for _, f := range schema.Fields {
					columns = append(columns, f.Name)
				}
				break
			}
			columns = append(columns, ident.Name)
		}
	}
	return columns
}

func (p *Planner) estimateRowCount(tableName string) int64 {
	if stats, ok := p.stats[tableName]; ok {
		return stats.RowCount
	}
	return 1000
}

func (p *Planner) UpdateStats(tableName string, stats *TableStats) {
	p.stats[tableName] = stats
}

func ExplainPlan(plan Plan, indent int) string {
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}

	result := prefix + "-> " + plan.Type()

	switch pl := plan.(type) {
	case *ScanPlan:
		result += " on " + pl.TableName
		if pl.IndexName != "" {
			result += " using " + pl.IndexName
		}
		result += fmt.Sprintf(" (cost=%.2f rows=%d)", pl.EstimatedCost, pl.EstimatedRows)
	case *FilterPlan:
		result += fmt.Sprintf(" (cost=%.2f)", pl.Cost())
	case *ProjectPlan:
		result += " (" + joinStrings(pl.Columns) + ")"
	case *SortPlan:
		result += fmt.Sprintf(" (cost=%.2f)", pl.EstimatedCost)
	case *LimitPlan:
		if pl.Offset > 0 {
			result += fmt.Sprintf(" limit=%d offset=%d", pl.Limit, pl.Offset)
		} else {
			result += fmt.Sprintf(" limit=%d", pl.Limit)
		}
	}

	result += "\n"
	for _, child := range plan.Children() {
		result += ExplainPlan(child, indent+1)
	}
	return result
}

func joinStrings(strs []string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
