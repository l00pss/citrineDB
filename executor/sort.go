package executor

import (
	"sort"
)

type SortDirection int

const (
	SortAsc SortDirection = iota
	SortDesc
)

type SortSpec struct {
	Column    string
	Direction SortDirection
}

type SortExecutor struct {
	columns  []string
	rows     [][]interface{}
	sortSpec []SortSpec
}

func NewSortExecutor() *SortExecutor {
	return &SortExecutor{}
}

func (s *SortExecutor) SetData(cols []string, rows [][]interface{}) {
	s.columns = cols
	s.rows = rows
}

func (s *SortExecutor) AddSort(column string, desc bool) {
	dir := SortAsc
	if desc {
		dir = SortDesc
	}
	s.sortSpec = append(s.sortSpec, SortSpec{Column: column, Direction: dir})
}

func (s *SortExecutor) Execute() [][]interface{} {
	if len(s.sortSpec) == 0 || len(s.rows) == 0 {
		return s.rows
	}

	result := make([][]interface{}, len(s.rows))
	copy(result, s.rows)

	sort.SliceStable(result, func(i, j int) bool {
		for _, spec := range s.sortSpec {
			idx := s.findColIndex(spec.Column)
			if idx < 0 {
				continue
			}
			cmp := compareVals(result[i][idx], result[j][idx])
			if cmp == 0 {
				continue
			}
			if spec.Direction == SortDesc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})

	return result
}

func (s *SortExecutor) findColIndex(name string) int {
	for i, c := range s.columns {
		if c == name {
			return i
		}
	}
	return -1
}

func compareVals(a, b interface{}) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	switch av := a.(type) {
	case int:
		bv, ok := b.(int)
		if ok {
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		}
	case int32:
		bv, ok := b.(int32)
		if ok {
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		}
	case int64:
		bv, ok := b.(int64)
		if ok {
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		}
	case float64:
		bv, ok := b.(float64)
		if ok {
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		}
	case string:
		bv, ok := b.(string)
		if ok {
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		}
	}
	return 0
}

type LimitExecutor struct {
	rows   [][]interface{}
	limit  int
	offset int
}

func NewLimitExecutor() *LimitExecutor {
	return &LimitExecutor{limit: -1}
}

func (l *LimitExecutor) SetData(rows [][]interface{}) {
	l.rows = rows
}

func (l *LimitExecutor) SetLimit(limit int) {
	l.limit = limit
}

func (l *LimitExecutor) SetOffset(offset int) {
	l.offset = offset
}

func (l *LimitExecutor) Execute() [][]interface{} {
	if len(l.rows) == 0 {
		return l.rows
	}

	start := l.offset
	if start < 0 {
		start = 0
	}
	if start >= len(l.rows) {
		return [][]interface{}{}
	}

	end := len(l.rows)
	if l.limit >= 0 {
		end = start + l.limit
		if end > len(l.rows) {
			end = len(l.rows)
		}
	}

	return l.rows[start:end]
}
