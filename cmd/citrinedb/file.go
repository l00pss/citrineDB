package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/l00pss/citrinedb/engine"
)

func readSQLFile(db *engine.DB, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	var buf strings.Builder
	executed := 0
	errors := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}

		if buf.Len() > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(line)

		if strings.HasSuffix(line, ";") {
			sql := strings.TrimSuffix(buf.String(), ";")
			sql = strings.TrimSpace(sql)

			if sql != "" {
				upper := strings.ToUpper(sql)
				if strings.HasPrefix(upper, "SELECT") {
					rows, err := db.Query(sql)
					if err != nil {
						fmt.Printf("Error: %v\n  SQL: %s\n", err, truncate(sql, 60))
						errors++
					} else {
						printRows(rows)
						executed++
					}
				} else {
					_, err := db.Execute(sql)
					if err != nil {
						fmt.Printf("Error: %v\n  SQL: %s\n", err, truncate(sql, 60))
						errors++
					} else {
						executed++
					}
				}
			}
			buf.Reset()
		}
	}

	fmt.Printf("\nExecuted: %d statements, Errors: %d\n", executed, errors)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
