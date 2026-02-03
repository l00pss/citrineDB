package executor

type JoinType int

const (
	InnerJoin JoinType = iota
	LeftJoin
	RightJoin
	FullJoin
	CrossJoin
)

func (j JoinType) String() string {
	switch j {
	case InnerJoin:
		return "INNER JOIN"
	case LeftJoin:
		return "LEFT JOIN"
	case RightJoin:
		return "RIGHT JOIN"
	case FullJoin:
		return "FULL JOIN"
	case CrossJoin:
		return "CROSS JOIN"
	default:
		return "UNKNOWN"
	}
}

type JoinExecutor struct {
	joinType   JoinType
	leftTable  string
	rightTable string
	leftRows   [][]interface{}
	rightRows  [][]interface{}
	leftCols   []string
	rightCols  []string
	onLeftCol  string
	onRightCol string
}

func NewJoinExecutor(jt JoinType) *JoinExecutor {
	return &JoinExecutor{joinType: jt}
}

func (j *JoinExecutor) SetLeftTable(name string, cols []string, rows [][]interface{}) {
	j.leftTable = name
	j.leftCols = cols
	j.leftRows = rows
}

func (j *JoinExecutor) SetRightTable(name string, cols []string, rows [][]interface{}) {
	j.rightTable = name
	j.rightCols = cols
	j.rightRows = rows
}

func (j *JoinExecutor) SetOnCondition(leftCol, rightCol string) {
	j.onLeftCol = leftCol
	j.onRightCol = rightCol
}

func (j *JoinExecutor) Execute() ([]string, [][]interface{}) {
	switch j.joinType {
	case LeftJoin:
		return j.executeLeftJoin()
	case RightJoin:
		return j.executeRightJoin()
	case CrossJoin:
		return j.executeCrossJoin()
	default:
		return j.executeInnerJoin()
	}
}

func (j *JoinExecutor) ResultColumns() []string {
	cols := make([]string, 0, len(j.leftCols)+len(j.rightCols))
	for _, c := range j.leftCols {
		cols = append(cols, j.leftTable+"."+c)
	}
	for _, c := range j.rightCols {
		cols = append(cols, j.rightTable+"."+c)
	}
	return cols
}

func (j *JoinExecutor) executeInnerJoin() ([]string, [][]interface{}) {
	cols := j.ResultColumns()
	var results [][]interface{}
	leftIdx := j.findColIndex(j.leftCols, j.onLeftCol)
	rightIdx := j.findColIndex(j.rightCols, j.onRightCol)
	if leftIdx < 0 || rightIdx < 0 {
		return cols, results
	}
	for _, lr := range j.leftRows {
		for _, rr := range j.rightRows {
			if j.valuesEqual(lr[leftIdx], rr[rightIdx]) {
				results = append(results, j.merge(lr, rr))
			}
		}
	}
	return cols, results
}

func (j *JoinExecutor) executeLeftJoin() ([]string, [][]interface{}) {
	cols := j.ResultColumns()
	var results [][]interface{}
	leftIdx := j.findColIndex(j.leftCols, j.onLeftCol)
	rightIdx := j.findColIndex(j.rightCols, j.onRightCol)
	if leftIdx < 0 || rightIdx < 0 {
		return cols, results
	}
	for _, lr := range j.leftRows {
		matched := false
		for _, rr := range j.rightRows {
			if j.valuesEqual(lr[leftIdx], rr[rightIdx]) {
				results = append(results, j.merge(lr, rr))
				matched = true
			}
		}
		if !matched {
			results = append(results, j.merge(lr, make([]interface{}, len(j.rightCols))))
		}
	}
	return cols, results
}

func (j *JoinExecutor) executeRightJoin() ([]string, [][]interface{}) {
	cols := j.ResultColumns()
	var results [][]interface{}
	leftIdx := j.findColIndex(j.leftCols, j.onLeftCol)
	rightIdx := j.findColIndex(j.rightCols, j.onRightCol)
	if leftIdx < 0 || rightIdx < 0 {
		return cols, results
	}
	for _, rr := range j.rightRows {
		matched := false
		for _, lr := range j.leftRows {
			if j.valuesEqual(lr[leftIdx], rr[rightIdx]) {
				results = append(results, j.merge(lr, rr))
				matched = true
			}
		}
		if !matched {
			results = append(results, j.merge(make([]interface{}, len(j.leftCols)), rr))
		}
	}
	return cols, results
}

func (j *JoinExecutor) executeCrossJoin() ([]string, [][]interface{}) {
	cols := j.ResultColumns()
	var results [][]interface{}
	for _, lr := range j.leftRows {
		for _, rr := range j.rightRows {
			results = append(results, j.merge(lr, rr))
		}
	}
	return cols, results
}

func (j *JoinExecutor) merge(l, r []interface{}) []interface{} {
	row := make([]interface{}, 0, len(l)+len(r))
	row = append(row, l...)
	row = append(row, r...)
	return row
}

func (j *JoinExecutor) findColIndex(cols []string, name string) int {
	for i, c := range cols {
		if c == name {
			return i
		}
	}
	return -1
}

func (j *JoinExecutor) valuesEqual(a, b interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	return toStr(a) == toStr(b)
}

func toStr(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int:
		return itoa(val)
	case int32:
		return itoa(int(val))
	case int64:
		return itoa(int(val))
	default:
		return ""
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
