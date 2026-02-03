package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/l00pss/citrinedb/engine"
)

func runREPL(db *engine.DB) {
	reader := bufio.NewReader(os.Stdin)
	var buf strings.Builder

	for {
		if buf.Len() == 0 {
			fmt.Print("citrinedb> ")
		} else {
			fmt.Print("      ...> ")
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, ".") && buf.Len() == 0 {
			handleDotCommand(db, line)
			continue
		}

		if buf.Len() > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(line)

		if strings.HasSuffix(line, ";") {
			executeSQL(db, buf.String())
			buf.Reset()
		}
	}
}

func executeSQL(db *engine.DB, sql string) {
	sql = strings.TrimSuffix(strings.TrimSpace(sql), ";")
	upper := strings.ToUpper(strings.TrimSpace(sql))

	if strings.HasPrefix(upper, "SELECT") {
		rows, err := db.Query(sql)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		printRows(rows)
	} else {
		res, err := db.Execute(sql)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if res.Message != "" {
			fmt.Println(res.Message)
		} else if res.RowsAffected > 0 {
			fmt.Printf("Rows affected: %d\n", res.RowsAffected)
		}
	}
}
