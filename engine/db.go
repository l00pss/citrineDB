package engine

import (
	"errors"
	"fmt"
	"sync"

	"github.com/l00pss/citrinedb/executor"
	"github.com/l00pss/citrinedb/storage/buffer"
	"github.com/l00pss/citrinedb/storage/catalog"
	"github.com/l00pss/citrinedb/storage/file"
	"github.com/l00pss/citrinedb/storage/tx"
)

var (
	ErrDatabaseClosed = errors.New("engine: database is closed")
	ErrDatabaseOpen   = errors.New("engine: database is already open")
)

// Config holds database configuration options
type Config struct {
	Path           string
	PageSize       int
	BufferPoolSize int
	WALDir         string
	SyncWrites     bool
}

// DefaultConfig returns a default configuration
func DefaultConfig(path string) Config {
	return Config{
		Path:           path,
		PageSize:       4096,
		BufferPoolSize: 1024,
		WALDir:         path + ".wal",
		SyncWrites:     true,
	}
}

// DB represents a CitrineDB database instance
type DB struct {
	mu          sync.RWMutex
	config      Config
	diskManager *file.DiskManager
	bufferPool  *buffer.BufferPool
	catalog     *catalog.Catalog
	walManager  *tx.WALManager
	executor    *executor.Executor
	closed      bool
}

// Open opens or creates a database at the given path
func Open(path string) (*DB, error) {
	return OpenWithConfig(DefaultConfig(path))
}

// OpenWithConfig opens or creates a database with custom configuration
func OpenWithConfig(config Config) (*DB, error) {
	dm, err := file.NewDiskManager(config.Path, file.Config{
		PageSize: config.PageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("engine: failed to open disk manager: %w", err)
	}

	bp := buffer.NewBufferPool(dm, buffer.Config{
		PoolSize: config.BufferPoolSize,
	})

	cat := catalog.NewCatalog(bp)
	if err := cat.Initialize(); err != nil {
		dm.Close()
		return nil, fmt.Errorf("engine: failed to initialize catalog: %w", err)
	}

	walConfig := tx.DefaultWALConfig()
	walConfig.Dir = config.WALDir
	walConfig.SyncAfterWrite = config.SyncWrites

	wal, err := tx.NewWALManager(walConfig)
	if err != nil {
		wal = nil
	}

	exec := executor.NewExecutor(cat)

	db := &DB{
		config:      config,
		diskManager: dm,
		bufferPool:  bp,
		catalog:     cat,
		walManager:  wal,
		executor:    exec,
		closed:      false,
	}

	return db, nil
}

// Close closes the database
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if err := db.bufferPool.FlushAll(); err != nil {
		return fmt.Errorf("engine: failed to flush buffer pool: %w", err)
	}

	if db.walManager != nil {
		if err := db.walManager.Close(); err != nil {
			return fmt.Errorf("engine: failed to close WAL: %w", err)
		}
	}

	if err := db.diskManager.Close(); err != nil {
		return fmt.Errorf("engine: failed to close disk manager: %w", err)
	}

	db.closed = true
	return nil
}

// Execute executes a SQL query and returns the result
func (db *DB) Execute(sql string) (*executor.Result, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, ErrDatabaseClosed
	}

	return db.executor.Execute(sql)
}

// Exec is an alias for Execute that returns rows affected
func (db *DB) Exec(sql string) (int64, error) {
	result, err := db.Execute(sql)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// Query executes a SELECT query and returns rows
func (db *DB) Query(sql string) (*Rows, error) {
	result, err := db.Execute(sql)
	if err != nil {
		return nil, err
	}

	return &Rows{
		columns: result.Columns,
		rows:    result.Rows,
		index:   -1,
	}, nil
}

// Catalog returns the database catalog
func (db *DB) Catalog() *catalog.Catalog {
	return db.catalog
}

// BufferPool returns the buffer pool
func (db *DB) BufferPool() *buffer.BufferPool {
	return db.bufferPool
}

// Stats returns database statistics
func (db *DB) Stats() *Stats {
	return &Stats{
		TableCount:     db.catalog.TableCount(),
		IndexCount:     db.catalog.IndexCount(),
		BufferPoolSize: db.config.BufferPoolSize,
		PageSize:       db.config.PageSize,
	}
}

// Stats holds database statistics
type Stats struct {
	TableCount     int
	IndexCount     int
	BufferPoolSize int
	PageSize       int
}

// Rows represents a result set from a query
type Rows struct {
	columns []string
	rows    [][]interface{}
	index   int
}

// Columns returns the column names
func (r *Rows) Columns() []string {
	return r.columns
}

// Next advances to the next row
func (r *Rows) Next() bool {
	r.index++
	return r.index < len(r.rows)
}

// Scan copies the current row values into dest
func (r *Rows) Scan(dest ...interface{}) error {
	if r.index < 0 || r.index >= len(r.rows) {
		return errors.New("engine: no current row")
	}

	row := r.rows[r.index]
	if len(dest) != len(row) {
		return fmt.Errorf("engine: expected %d destinations, got %d", len(row), len(dest))
	}

	for i, val := range row {
		if err := scanValue(dest[i], val); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the Rows
func (r *Rows) Close() error {
	return nil
}

// Row returns the current row values
func (r *Rows) Row() []interface{} {
	if r.index < 0 || r.index >= len(r.rows) {
		return nil
	}
	return r.rows[r.index]
}

// Count returns the total number of rows
func (r *Rows) Count() int {
	return len(r.rows)
}

// Values returns the current row values (alias for Row)
func (r *Rows) Values() []interface{} {
	return r.Row()
}

func scanValue(dest, src interface{}) error {
	switch d := dest.(type) {
	case *int:
		switch s := src.(type) {
		case int:
			*d = s
		case int8:
			*d = int(s)
		case int16:
			*d = int(s)
		case int32:
			*d = int(s)
		case int64:
			*d = int(s)
		case float64:
			*d = int(s)
		default:
			return fmt.Errorf("engine: cannot scan %T into *int", src)
		}
	case *int32:
		switch s := src.(type) {
		case int32:
			*d = s
		case int:
			*d = int32(s)
		case int64:
			*d = int32(s)
		case float64:
			*d = int32(s)
		default:
			return fmt.Errorf("engine: cannot scan %T into *int32", src)
		}
	case *int64:
		switch s := src.(type) {
		case int64:
			*d = s
		case int:
			*d = int64(s)
		case int32:
			*d = int64(s)
		case float64:
			*d = int64(s)
		default:
			return fmt.Errorf("engine: cannot scan %T into *int64", src)
		}
	case *float64:
		switch s := src.(type) {
		case float64:
			*d = s
		case float32:
			*d = float64(s)
		case int:
			*d = float64(s)
		case int64:
			*d = float64(s)
		default:
			return fmt.Errorf("engine: cannot scan %T into *float64", src)
		}
	case *string:
		switch s := src.(type) {
		case string:
			*d = s
		default:
			*d = fmt.Sprintf("%v", src)
		}
	case *bool:
		switch s := src.(type) {
		case bool:
			*d = s
		default:
			return fmt.Errorf("engine: cannot scan %T into *bool", src)
		}
	case *interface{}:
		*d = src
	default:
		return fmt.Errorf("engine: unsupported destination type %T", dest)
	}

	return nil
}
