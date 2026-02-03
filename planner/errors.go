package planner

import "errors"

var (
	ErrNoFromClause  = errors.New("planner: SELECT requires FROM clause")
	ErrTableNotFound = errors.New("planner: table not found")
	ErrInvalidPlan   = errors.New("planner: invalid execution plan")
)
