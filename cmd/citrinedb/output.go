package main

import (
	"fmt"
	"strings"

	"github.com/l00pss/citrinedb/engine"
)

func printRows(rows *engine.Rows) {
	cols := rows.Columns()
	if len(cols) == 0 {
		return
	}

	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c)
	}

	var data [][]string
	for rows.Next() {
		r := rows.Row()
		strs := make([]string, len(r))
		for i, v := range r {
			if v == nil {
				strs[i] = "NULL"
			} else {
				strs[i] = fmt.Sprintf("%v", v)
			}
			if len(strs[i]) > widths[i] {
				widths[i] = len(strs[i])
			}
		}
		data = append(data, strs)
	}

	for i, c := range cols {
		if i > 0 {
			fmt.Print(" | ")
		}
		fmt.Printf("%-*s", widths[i], c)
	}
	fmt.Println()

	for i, w := range widths {
		if i > 0 {
			fmt.Print("-+-")
		}
		fmt.Print(strings.Repeat("-", w))
	}
	fmt.Println()

	for _, row := range data {
		for i, val := range row {
			if i > 0 {
				fmt.Print(" | ")
			}
			fmt.Printf("%-*s", widths[i], val)
		}
		fmt.Println()
	}
	fmt.Printf("(%d rows)\n", len(data))
}
