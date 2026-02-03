package catalog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/l00pss/citrinedb/storage/buffer"
	"github.com/l00pss/citrinedb/storage/index"
	"github.com/l00pss/citrinedb/storage/page"
	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/citrinedb/storage/table"
)

var (
	ErrTableNotFound      = errors.New("catalog: table not found")
	ErrTableExists        = errors.New("catalog: table already exists")
	ErrIndexNotFound      = errors.New("catalog: index not found")
	ErrIndexExists        = errors.New("catalog: index already exists")
	ErrInvalidCatalog     = errors.New("catalog: invalid catalog data")
	ErrColumnNotFound     = errors.New("catalog: column not found")
	ErrInvalidColumnCount = errors.New("catalog: invalid column count")
)

const (
	SystemTableName  = "_citrinedb_tables"
	SystemIndexName  = "_citrinedb_indexes"
	SystemColumnName = "_citrinedb_columns"
	CatalogVersion   = 1
	MaxTableNameLen  = 128
	MaxColumnNameLen = 128
	MaxIndexNameLen  = 128
)

// TableID is a unique identifier for a table
type TableID uint32

// IndexID is a unique identifier for an index
type IndexID uint32

// TableInfo holds metadata about a table
type TableInfo struct {
	ID          TableID
	Name        string
	Schema      *record.Schema
	HeapFile    *table.HeapFile
	Indexes     map[string]*IndexInfo
	CreatedAt   time.Time
	FirstPageID page.PageID
}

// IndexInfo holds metadata about an index
type IndexInfo struct {
	ID        IndexID
	Name      string
	TableID   TableID
	TableName string
	Columns   []string
	Unique    bool
	BTree     *index.BTreeIndex
	CreatedAt time.Time
}

// ColumnInfo holds metadata about a column (for persistence)
type ColumnInfo struct {
	TableID  TableID
	Name     string
	Type     record.FieldType
	Position int
	Nullable bool
	MaxLen   int
}

// Catalog manages all database metadata
type Catalog struct {
	mu           sync.RWMutex
	bufferPool   *buffer.BufferPool
	tables       map[string]*TableInfo
	tablesByID   map[TableID]*TableInfo
	indexes      map[string]*IndexInfo
	indexesByID  map[IndexID]*IndexInfo
	nextTableID  TableID
	nextIndexID  IndexID
	systemTable  *table.HeapFile
	systemIndex  *table.HeapFile
	systemColumn *table.HeapFile
	initialized  bool
}

// NewCatalog creates a new catalog manager
func NewCatalog(bp *buffer.BufferPool) *Catalog {
	return &Catalog{
		bufferPool:  bp,
		tables:      make(map[string]*TableInfo),
		tablesByID:  make(map[TableID]*TableInfo),
		indexes:     make(map[string]*IndexInfo),
		indexesByID: make(map[IndexID]*IndexInfo),
		nextTableID: 1,
		nextIndexID: 1,
	}
}

// Initialize sets up the catalog (creates system tables if needed)
func (c *Catalog) Initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	// Create system table schema for storing table metadata
	tableSchema := record.NewSchema([]record.Field{
		{Name: "table_id", Type: record.FieldTypeInt32, Nullable: false},
		{Name: "table_name", Type: record.FieldTypeString, Nullable: false, MaxLen: MaxTableNameLen},
		{Name: "first_page_id", Type: record.FieldTypeInt32, Nullable: false},
		{Name: "created_at", Type: record.FieldTypeInt64, Nullable: false},
	})

	// Create system index schema
	indexSchema := record.NewSchema([]record.Field{
		{Name: "index_id", Type: record.FieldTypeInt32, Nullable: false},
		{Name: "index_name", Type: record.FieldTypeString, Nullable: false, MaxLen: MaxIndexNameLen},
		{Name: "table_id", Type: record.FieldTypeInt32, Nullable: false},
		{Name: "is_unique", Type: record.FieldTypeBool, Nullable: false},
		{Name: "columns", Type: record.FieldTypeString, Nullable: false},
		{Name: "created_at", Type: record.FieldTypeInt64, Nullable: false},
	})

	// Create system column schema
	columnSchema := record.NewSchema([]record.Field{
		{Name: "table_id", Type: record.FieldTypeInt32, Nullable: false},
		{Name: "column_name", Type: record.FieldTypeString, Nullable: false, MaxLen: MaxColumnNameLen},
		{Name: "column_type", Type: record.FieldTypeInt8, Nullable: false},
		{Name: "position", Type: record.FieldTypeInt16, Nullable: false},
		{Name: "nullable", Type: record.FieldTypeBool, Nullable: false},
		{Name: "max_len", Type: record.FieldTypeInt32, Nullable: false},
	})

	// Create system heap files
	var err error
	c.systemTable, err = table.NewHeapFile(table.HeapFileConfig{
		TableID:    0,
		Schema:     tableSchema,
		BufferPool: c.bufferPool,
	})
	if err != nil {
		return fmt.Errorf("catalog: failed to create system table: %w", err)
	}

	c.systemIndex, err = table.NewHeapFile(table.HeapFileConfig{
		TableID:    0,
		Schema:     indexSchema,
		BufferPool: c.bufferPool,
	})
	if err != nil {
		return fmt.Errorf("catalog: failed to create system index table: %w", err)
	}

	c.systemColumn, err = table.NewHeapFile(table.HeapFileConfig{
		TableID:    0,
		Schema:     columnSchema,
		BufferPool: c.bufferPool,
	})
	if err != nil {
		return fmt.Errorf("catalog: failed to create system column table: %w", err)
	}

	c.initialized = true
	return nil
}

// CreateTable creates a new table with the given schema
func (c *Catalog) CreateTable(name string, schema *record.Schema) (*TableInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return nil, ErrInvalidCatalog
	}

	if _, exists := c.tables[name]; exists {
		return nil, ErrTableExists
	}

	// Create heap file for the table
	heapFile, err := table.NewHeapFile(table.HeapFileConfig{
		TableID:    table.TableID(c.nextTableID),
		Schema:     schema,
		BufferPool: c.bufferPool,
	})
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to create heap file: %w", err)
	}

	tableInfo := &TableInfo{
		ID:          c.nextTableID,
		Name:        name,
		Schema:      schema,
		HeapFile:    heapFile,
		Indexes:     make(map[string]*IndexInfo),
		CreatedAt:   time.Now(),
		FirstPageID: heapFile.FirstPageID(),
	}

	// Store in memory
	c.tables[name] = tableInfo
	c.tablesByID[c.nextTableID] = tableInfo
	c.nextTableID++

	// Persist table metadata
	if err := c.persistTableInfo(tableInfo); err != nil {
		delete(c.tables, name)
		delete(c.tablesByID, tableInfo.ID)
		c.nextTableID--
		return nil, err
	}

	// Persist column metadata
	for i, field := range schema.Fields {
		colInfo := ColumnInfo{
			TableID:  tableInfo.ID,
			Name:     field.Name,
			Type:     field.Type,
			Position: i,
			Nullable: field.Nullable,
			MaxLen:   field.MaxLen,
		}
		if err := c.persistColumnInfo(colInfo); err != nil {
			return nil, err
		}
	}

	return tableInfo, nil
}

// GetTable returns table info by name
func (c *Catalog) GetTable(name string) (*TableInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tableInfo, exists := c.tables[name]
	if !exists {
		return nil, ErrTableNotFound
	}
	return tableInfo, nil
}

// GetTableByID returns table info by ID
func (c *Catalog) GetTableByID(id TableID) (*TableInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tableInfo, exists := c.tablesByID[id]
	if !exists {
		return nil, ErrTableNotFound
	}
	return tableInfo, nil
}

// DropTable removes a table and its indexes
func (c *Catalog) DropTable(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tableInfo, exists := c.tables[name]
	if !exists {
		return ErrTableNotFound
	}

	// Remove all indexes for this table
	for indexName := range tableInfo.Indexes {
		delete(c.indexes, indexName)
	}

	// Remove from maps
	delete(c.tables, name)
	delete(c.tablesByID, tableInfo.ID)

	return nil
}

// CreateIndex creates a new index on a table
func (c *Catalog) CreateIndex(name, tableName string, columns []string, unique bool) (*IndexInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return nil, ErrInvalidCatalog
	}

	if _, exists := c.indexes[name]; exists {
		return nil, ErrIndexExists
	}

	tableInfo, exists := c.tables[tableName]
	if !exists {
		return nil, ErrTableNotFound
	}

	// Validate columns exist
	for _, col := range columns {
		if _, ok := tableInfo.Schema.FieldIndex(col); !ok {
			return nil, ErrColumnNotFound
		}
	}

	// Create B+Tree index
	btree := index.NewBTreeIndex(index.IndexConfig{
		ID:     index.IndexID(c.nextIndexID),
		Name:   name,
		Unique: unique,
		Degree: 50,
	})

	indexInfo := &IndexInfo{
		ID:        c.nextIndexID,
		Name:      name,
		TableID:   tableInfo.ID,
		TableName: tableName,
		Columns:   columns,
		Unique:    unique,
		BTree:     btree,
		CreatedAt: time.Now(),
	}

	// Store in memory
	c.indexes[name] = indexInfo
	c.indexesByID[c.nextIndexID] = indexInfo
	tableInfo.Indexes[name] = indexInfo
	c.nextIndexID++

	// Persist index metadata
	if err := c.persistIndexInfo(indexInfo); err != nil {
		delete(c.indexes, name)
		delete(c.indexesByID, indexInfo.ID)
		delete(tableInfo.Indexes, name)
		c.nextIndexID--
		return nil, err
	}

	return indexInfo, nil
}

// GetIndex returns index info by name
func (c *Catalog) GetIndex(name string) (*IndexInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	indexInfo, exists := c.indexes[name]
	if !exists {
		return nil, ErrIndexNotFound
	}
	return indexInfo, nil
}

// DropIndex removes an index
func (c *Catalog) DropIndex(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexInfo, exists := c.indexes[name]
	if !exists {
		return ErrIndexNotFound
	}

	// Remove from table's index map
	if tableInfo, ok := c.tablesByID[indexInfo.TableID]; ok {
		delete(tableInfo.Indexes, name)
	}

	delete(c.indexes, name)
	delete(c.indexesByID, indexInfo.ID)

	return nil
}

// ListTables returns all table names
func (c *Catalog) ListTables() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.tables))
	for name := range c.tables {
		names = append(names, name)
	}
	return names
}

// ListIndexes returns all index names
func (c *Catalog) ListIndexes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.indexes))
	for name := range c.indexes {
		names = append(names, name)
	}
	return names
}

// TableCount returns the number of tables
func (c *Catalog) TableCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.tables)
}

// IndexCount returns the number of indexes
func (c *Catalog) IndexCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.indexes)
}

// persistTableInfo saves table metadata to system table
func (c *Catalog) persistTableInfo(ti *TableInfo) error {
	rec := record.NewRecord(c.systemTable.Schema())
	rec.Set(0, record.Int32Value(int32(ti.ID)))
	rec.Set(1, record.StringValue(ti.Name))
	rec.Set(2, record.Int32Value(int32(ti.FirstPageID)))
	rec.Set(3, record.Int64Value(ti.CreatedAt.UnixNano()))

	_, err := c.systemTable.Insert(rec)
	return err
}

// persistIndexInfo saves index metadata to system index table
func (c *Catalog) persistIndexInfo(ii *IndexInfo) error {
	columnsStr := serializeColumns(ii.Columns)

	rec := record.NewRecord(c.systemIndex.Schema())
	rec.Set(0, record.Int32Value(int32(ii.ID)))
	rec.Set(1, record.StringValue(ii.Name))
	rec.Set(2, record.Int32Value(int32(ii.TableID)))
	rec.Set(3, record.BoolValue(ii.Unique))
	rec.Set(4, record.StringValue(columnsStr))
	rec.Set(5, record.Int64Value(ii.CreatedAt.UnixNano()))

	_, err := c.systemIndex.Insert(rec)
	return err
}

// persistColumnInfo saves column metadata to system column table
func (c *Catalog) persistColumnInfo(ci ColumnInfo) error {
	rec := record.NewRecord(c.systemColumn.Schema())
	rec.Set(0, record.Int32Value(int32(ci.TableID)))
	rec.Set(1, record.StringValue(ci.Name))
	rec.Set(2, record.Int8Value(int8(ci.Type)))
	rec.Set(3, record.Int16Value(int16(ci.Position)))
	rec.Set(4, record.BoolValue(ci.Nullable))
	rec.Set(5, record.Int32Value(int32(ci.MaxLen)))

	_, err := c.systemColumn.Insert(rec)
	return err
}

func serializeColumns(cols []string) string {
	result := ""
	for i, col := range cols {
		if i > 0 {
			result += ","
		}
		result += col
	}
	return result
}

func deserializeColumns(s string) []string {
	if s == "" {
		return nil
	}
	var cols []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			cols = append(cols, s[start:i])
			start = i + 1
		}
	}
	return cols
}

// SerializeTableID converts TableID to bytes
func SerializeTableID(id TableID) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(id))
	return buf
}

// DeserializeTableID converts bytes to TableID
func DeserializeTableID(data []byte) TableID {
	return TableID(binary.LittleEndian.Uint32(data))
}
