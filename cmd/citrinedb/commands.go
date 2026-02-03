package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/l00pss/citrinedb/engine"
)

func handleDotCommand(db *engine.DB, cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}
	c := strings.ToLower(parts[0])

	switch c {
	case ".help":
		printHelp()
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
		showStats(db)
	case ".read":
		if len(parts) > 1 {
			readSQLFile(db, parts[1])
		} else {
			fmt.Println("Usage: .read FILE")
		}
	default:
		fmt.Printf("Unknown: %s\n", c)
	}
}

func printHelp() {
	fmt.Println(".help         Show help")
	fmt.Println(".quit         Exit")
	fmt.Println(".tables       List tables")
	fmt.Println(".schema TABLE Show schema")
	fmt.Println(".stats        Show stats")
	fmt.Println(".read FILE    Execute SQL file")
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

func showStats(db *engine.DB) {
	s := db.Stats()
	fmt.Printf("Tables: %d, Indexes: %d\n", s.TableCount, s.IndexCount)
	fmt.Printf("Page Size: %d, Buffer Pool: %d\n", s.PageSize, s.BufferPoolSize)
}
