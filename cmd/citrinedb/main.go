package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/l00pss/citrinedb/engine"
)

const version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("CitrineDB %s\n", version)
		return
	}

	args := flag.Args()
	dbPath := ":memory:"
	if len(args) > 0 {
		dbPath = args[0]
	}

	fmt.Printf("CitrineDB %s\n", version)

	var db *engine.DB
	var err error
	var tmpDir string

	if dbPath == ":memory:" {
		tmpDir, _ = os.MkdirTemp("", "citrinedb-")
		dbPath = tmpDir + "/memory.db"
		fmt.Println("Connected to in-memory database.")
	} else {
		fmt.Printf("Connected to: %s\n", dbPath)
	}

	db, err = engine.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		db.Close()
		if tmpDir != "" {
			os.RemoveAll(tmpDir)
		}
	}()

	fmt.Println("Type .help for commands.")
	repl(db)
}

func repl(db *engine.DB) {
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
			dotCmd(db, line)
			continue
		}

		if buf.Len() > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(line)

		if strings.HasSuffix(line, ";") {
			execSQL(db, buf.String())
			buf.Reset()
		}
	}
}

func dotCmd(db *engine.DB, cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}
	c := strings.ToLower(parts[0])

	switch c {
	case ".help":
		fmt.Println(".help    Show help")
		fmt.Println(".quit    Exit")
		fmt.Println(".tables  List tables")
		fmt.Println(".schema  Show schema")
		fmt.Println(".stats   Show stats")
	case ".quit", ".exit":
		fmt.Println("Bye!")
		os.Exit(0)
	case ".tables":
		showTables(db)
	case ".schema":
		if len(parts) > 1 {
			showSchema(db, parts[1])
		} else {
			fmt.Println("Usage: .schema TABLE")
		}
	case ".stats":
		s := db.Stats()
		fmt.Printf("Tables: %d, Indexes: %d\n", s.TableCount, s.IndexCount)
		fmt.Printf("Page Size: %d, Buffer Pool: %d\n", s.PageSize, s.BufferPoolSize)
	default:
		fmt.Printf("Unknown: %s\n", c)
	}
}

func showTables(db *engine.DB) {
	s := db.Stats()
	if s.TableCount == 0 {
		fmt.Println("(no tables)")
		return
	}
	rows, err := db.Query("SELECT name FROM __tables__")
	if err != nil {
		fmt.Printf("(%d tables)\n", s.TableCount)
		return
	}
	for rows.Next() {
		r := rows.Row()
		if len(r) > 0 {
			fmt.Println(r[0])
		}
	}
}

func showSchema(db *engine.DB, name string) {
	res, err := db.Execute("SELECT * FROM " + name + " LIMIT 0")
	if err != nil {
		fmt.Printf("Table not found: %s\n", name)
		return
	}
	fmt.Printf("CREATE TABLE %s (", name)
	for i, col := range res.Columns {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(col)
	}
	fmt.Println(");")
}

func execSQL(db *engine.DB, sql string) {
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
