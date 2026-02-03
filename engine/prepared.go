package engine

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/l00pss/citrinedb/executor"
)

var (
	ErrInvalidPlaceholder   = errors.New("prepared: invalid placeholder")
	ErrMissingParameter     = errors.New("prepared: missing parameter")
	ErrTooManyParameters    = errors.New("prepared: too many parameters")
	ErrInvalidParameterType = errors.New("prepared: invalid parameter type")
)

// PreparedStatement represents a prepared SQL statement with parameter placeholders
type PreparedStatement struct {
	db            *DB
	originalSQL   string
	placeholders  []placeholder
	paramCount    int
	placeholderRe *regexp.Regexp
}

type placeholder struct {
	position int    // position in SQL string
	length   int    // length of placeholder (1 for ?, variable for $1, $2, etc.)
	index    int    // 0-based parameter index
	style    string // "?" or "$"
}

// Prepare creates a new prepared statement
// Supports both ? (positional) and $1, $2, ... (numbered) placeholders
func (db *DB) Prepare(sql string) (*PreparedStatement, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, ErrDatabaseClosed
	}

	stmt := &PreparedStatement{
		db:          db,
		originalSQL: sql,
	}

	if err := stmt.parsePlaceholders(); err != nil {
		return nil, err
	}

	return stmt, nil
}

// parsePlaceholders finds and validates all placeholders in the SQL
func (stmt *PreparedStatement) parsePlaceholders() error {
	sql := stmt.originalSQL
	var placeholders []placeholder
	var style string

	// Check for numbered placeholders ($1, $2, etc.)
	numberedRe := regexp.MustCompile(`\$(\d+)`)
	numberedMatches := numberedRe.FindAllStringSubmatchIndex(sql, -1)

	// Check for positional placeholders (?)
	positionalCount := strings.Count(sql, "?")

	// Validate - can't mix styles
	if len(numberedMatches) > 0 && positionalCount > 0 {
		return fmt.Errorf("%w: cannot mix ? and $N placeholder styles", ErrInvalidPlaceholder)
	}

	if len(numberedMatches) > 0 {
		// Numbered style ($1, $2, ...)
		style = "$"
		maxIndex := 0

		for _, match := range numberedMatches {
			numStr := sql[match[2]:match[3]]
			num, err := strconv.Atoi(numStr)
			if err != nil || num < 1 {
				return fmt.Errorf("%w: $%s is not valid", ErrInvalidPlaceholder, numStr)
			}
			if num > maxIndex {
				maxIndex = num
			}
			placeholders = append(placeholders, placeholder{
				position: match[0],
				length:   match[1] - match[0],
				index:    num - 1, // Convert to 0-based
				style:    style,
			})
		}
		stmt.paramCount = maxIndex
	} else if positionalCount > 0 {
		// Positional style (?)
		style = "?"
		index := 0
		pos := 0

		for {
			idx := strings.Index(sql[pos:], "?")
			if idx == -1 {
				break
			}
			// Check if ? is inside a string literal
			if !isInsideString(sql, pos+idx) {
				placeholders = append(placeholders, placeholder{
					position: pos + idx,
					length:   1,
					index:    index,
					style:    style,
				})
				index++
			}
			pos += idx + 1
		}
		stmt.paramCount = index
	}

	stmt.placeholders = placeholders
	return nil
}

// isInsideString checks if a position is inside a string literal
func isInsideString(sql string, pos int) bool {
	inString := false
	stringChar := byte(0)

	for i := 0; i < pos; i++ {
		c := sql[i]
		if !inString {
			if c == '\'' || c == '"' {
				inString = true
				stringChar = c
			}
		} else {
			if c == stringChar {
				// Check for escaped quote
				if i+1 < len(sql) && sql[i+1] == stringChar {
					i++ // Skip escaped quote
				} else {
					inString = false
				}
			}
		}
	}

	return inString
}

// Execute executes the prepared statement with the given parameters
func (stmt *PreparedStatement) Execute(args ...interface{}) (*executor.Result, error) {
	sql, err := stmt.bindParameters(args...)
	if err != nil {
		return nil, err
	}

	return stmt.db.Execute(sql)
}

// Query executes the prepared statement and returns rows
func (stmt *PreparedStatement) Query(args ...interface{}) (*Rows, error) {
	sql, err := stmt.bindParameters(args...)
	if err != nil {
		return nil, err
	}

	return stmt.db.Query(sql)
}

// Exec executes the prepared statement and returns rows affected
func (stmt *PreparedStatement) Exec(args ...interface{}) (int64, error) {
	result, err := stmt.Execute(args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// bindParameters replaces placeholders with sanitized parameter values
func (stmt *PreparedStatement) bindParameters(args ...interface{}) (string, error) {
	if len(args) < stmt.paramCount {
		return "", fmt.Errorf("%w: expected %d, got %d", ErrMissingParameter, stmt.paramCount, len(args))
	}
	if len(args) > stmt.paramCount {
		return "", fmt.Errorf("%w: expected %d, got %d", ErrTooManyParameters, stmt.paramCount, len(args))
	}

	if len(stmt.placeholders) == 0 {
		return stmt.originalSQL, nil
	}

	// Build the final SQL by replacing placeholders from end to start
	// (to preserve positions)
	sql := stmt.originalSQL

	// Sort placeholders by position descending
	sorted := make([]placeholder, len(stmt.placeholders))
	copy(sorted, stmt.placeholders)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].position < sorted[j].position {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	for _, ph := range sorted {
		if ph.index >= len(args) {
			return "", fmt.Errorf("%w: parameter $%d", ErrMissingParameter, ph.index+1)
		}

		value := args[ph.index]
		sanitized, err := sanitizeValue(value)
		if err != nil {
			return "", err
		}

		sql = sql[:ph.position] + sanitized + sql[ph.position+ph.length:]
	}

	return sql, nil
}

// sanitizeValue converts a Go value to a safe SQL literal
func sanitizeValue(v interface{}) (string, error) {
	if v == nil {
		return "NULL", nil
	}

	switch val := v.(type) {
	case bool:
		if val {
			return "TRUE", nil
		}
		return "FALSE", nil

	case int:
		return strconv.Itoa(val), nil
	case int8:
		return strconv.FormatInt(int64(val), 10), nil
	case int16:
		return strconv.FormatInt(int64(val), 10), nil
	case int32:
		return strconv.FormatInt(int64(val), 10), nil
	case int64:
		return strconv.FormatInt(val, 10), nil

	case uint:
		return strconv.FormatUint(uint64(val), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(val), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(val), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(val), 10), nil
	case uint64:
		return strconv.FormatUint(val, 10), nil

	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), nil

	case string:
		return escapeString(val), nil

	case []byte:
		return escapeBytes(val), nil

	default:
		return "", fmt.Errorf("%w: %T", ErrInvalidParameterType, v)
	}
}

// escapeString escapes a string for safe SQL inclusion
func escapeString(s string) string {
	// SQL standard: escape single quotes by doubling them
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}

// escapeBytes converts bytes to a hex literal
func escapeBytes(b []byte) string {
	return fmt.Sprintf("X'%X'", b)
}

// ParamCount returns the number of parameters expected
func (stmt *PreparedStatement) ParamCount() int {
	return stmt.paramCount
}

// SQL returns the original SQL template
func (stmt *PreparedStatement) SQL() string {
	return stmt.originalSQL
}

// Close closes the prepared statement (no-op for now, but good practice)
func (stmt *PreparedStatement) Close() error {
	stmt.db = nil
	return nil
}
