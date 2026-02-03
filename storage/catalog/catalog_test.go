package catalog

import (
	"os"
	"testing"

	"github.com/l00pss/citrinedb/storage/buffer"
	"github.com/l00pss/citrinedb/storage/file"
	"github.com/l00pss/citrinedb/storage/record"
)

func setupTestCatalog(t *testing.T) (*Catalog, func()) {
	t.Helper()

	dbPath := t.TempDir() + "/test.db"
	dm, err := file.NewDiskManager(dbPath, file.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}

	bp := buffer.NewBufferPool(dm, buffer.Config{PoolSize: 100})
	catalog := NewCatalog(bp)

	cleanup := func() {
		bp.FlushAll()
		dm.Close()
		os.Remove(dbPath)
	}

	return catalog, cleanup
}

func TestCatalogInitialize(t *testing.T) {
	catalog, cleanup := setupTestCatalog(t)
	defer cleanup()

	err := catalog.Initialize()
	if err != nil {
		t.Fatalf("failed to initialize catalog: %v", err)
	}

	if !catalog.initialized {
		t.Error("catalog should be initialized")
	}

	err = catalog.Initialize()
	if err != nil {
		t.Fatalf("second initialize failed: %v", err)
	}
}

func TestCreateTable(t *testing.T) {
	catalog, cleanup := setupTestCatalog(t)
	defer cleanup()

	if err := catalog.Initialize(); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt32, Nullable: false},
		{Name: "name", Type: record.FieldTypeString, Nullable: false, MaxLen: 100},
		{Name: "age", Type: record.FieldTypeInt16, Nullable: true},
	})

	tableInfo, err := catalog.CreateTable("users", schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	if tableInfo.Name != "users" {
		t.Errorf("expected table name 'users', got '%s'", tableInfo.Name)
	}

	if tableInfo.ID != 1 {
		t.Errorf("expected table ID 1, got %d", tableInfo.ID)
	}

	if tableInfo.Schema.FieldCount() != 3 {
		t.Errorf("expected 3 fields, got %d", tableInfo.Schema.FieldCount())
	}

	if catalog.TableCount() != 1 {
		t.Errorf("expected 1 table, got %d", catalog.TableCount())
	}
}

func TestCreateTableDuplicate(t *testing.T) {
	catalog, cleanup := setupTestCatalog(t)
	defer cleanup()

	if err := catalog.Initialize(); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt32, Nullable: false},
	})

	_, err := catalog.CreateTable("users", schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = catalog.CreateTable("users", schema)
	if err != ErrTableExists {
		t.Errorf("expected ErrTableExists, got %v", err)
	}
}

func TestGetTable(t *testing.T) {
	catalog, cleanup := setupTestCatalog(t)
	defer cleanup()

	if err := catalog.Initialize(); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt32, Nullable: false},
	})

	created, err := catalog.CreateTable("users", schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	fetched, err := catalog.GetTable("users")
	if err != nil {
		t.Fatalf("failed to get table: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("table IDs don't match")
	}

	fetchedByID, err := catalog.GetTableByID(created.ID)
	if err != nil {
		t.Fatalf("failed to get table by ID: %v", err)
	}

	if fetchedByID.Name != "users" {
		t.Errorf("expected name 'users', got '%s'", fetchedByID.Name)
	}

	_, err = catalog.GetTable("nonexistent")
	if err != ErrTableNotFound {
		t.Errorf("expected ErrTableNotFound, got %v", err)
	}
}

func TestDropTable(t *testing.T) {
	catalog, cleanup := setupTestCatalog(t)
	defer cleanup()

	if err := catalog.Initialize(); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt32, Nullable: false},
	})

	_, err := catalog.CreateTable("users", schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	err = catalog.DropTable("users")
	if err != nil {
		t.Fatalf("failed to drop table: %v", err)
	}

	if catalog.TableCount() != 0 {
		t.Errorf("expected 0 tables, got %d", catalog.TableCount())
	}

	err = catalog.DropTable("nonexistent")
	if err != ErrTableNotFound {
		t.Errorf("expected ErrTableNotFound, got %v", err)
	}
}

func TestCreateIndex(t *testing.T) {
	catalog, cleanup := setupTestCatalog(t)
	defer cleanup()

	if err := catalog.Initialize(); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt32, Nullable: false},
		{Name: "name", Type: record.FieldTypeString, Nullable: false, MaxLen: 100},
	})

	_, err := catalog.CreateTable("users", schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	indexInfo, err := catalog.CreateIndex("idx_users_id", "users", []string{"id"}, true)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	if indexInfo.Name != "idx_users_id" {
		t.Errorf("expected index name 'idx_users_id', got '%s'", indexInfo.Name)
	}

	if !indexInfo.Unique {
		t.Error("expected unique index")
	}

	if catalog.IndexCount() != 1 {
		t.Errorf("expected 1 index, got %d", catalog.IndexCount())
	}
}

func TestDropIndex(t *testing.T) {
	catalog, cleanup := setupTestCatalog(t)
	defer cleanup()

	if err := catalog.Initialize(); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt32, Nullable: false},
	})

	_, err := catalog.CreateTable("users", schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = catalog.CreateIndex("idx_id", "users", []string{"id"}, true)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	err = catalog.DropIndex("idx_id")
	if err != nil {
		t.Fatalf("failed to drop index: %v", err)
	}

	if catalog.IndexCount() != 0 {
		t.Errorf("expected 0 indexes, got %d", catalog.IndexCount())
	}
}

func TestListTables(t *testing.T) {
	catalog, cleanup := setupTestCatalog(t)
	defer cleanup()

	if err := catalog.Initialize(); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt32, Nullable: false},
	})

	catalog.CreateTable("users", schema)
	catalog.CreateTable("posts", schema)
	catalog.CreateTable("comments", schema)

	tables := catalog.ListTables()
	if len(tables) != 3 {
		t.Errorf("expected 3 tables, got %d", len(tables))
	}
}

func TestSerializeDeserializeColumns(t *testing.T) {
	cols := []string{"id", "name", "age"}
	serialized := serializeColumns(cols)
	deserialized := deserializeColumns(serialized)

	if len(deserialized) != len(cols) {
		t.Errorf("expected %d columns, got %d", len(cols), len(deserialized))
	}

	for i, col := range cols {
		if deserialized[i] != col {
			t.Errorf("expected column '%s', got '%s'", col, deserialized[i])
		}
	}
}
