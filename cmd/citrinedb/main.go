package main

import (
	"flag"
	"fmt"
	"os"

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
	runREPL(db)
}
