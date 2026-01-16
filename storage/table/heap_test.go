package table

import (
	"path/filepath"
	"testing"

	"github.com/l00pss/citrinedb/storage/buffer"
	"github.com/l00pss/citrinedb/storage/file"
	"github.com/l00pss/citrinedb/storage/record"
)

func setupHeapFile(t *testing.T) (*HeapFile, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dm, err := file.NewDiskManager(dbPath, file.DefaultConfig())
	if err != nil {
		t.Fatalf("disk manager: %v", err)
	}
	bp := buffer.NewBufferPool(dm, buffer.Config{PoolSize: 100})
	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt64},
		{Name: "name", Type: record.FieldTypeString},
		{Name: "age", Type: record.FieldTypeInt32},
	})
	hf, err := NewHeapFile(HeapFileConfig{TableID: 1, Schema: schema, BufferPool: bp})
	if err != nil {
		t.Fatalf("heap file: %v", err)
	}
	return hf, func() { bp.FlushAll(); dm.Close() }
}

func makeRecord(s *record.Schema, id int64, name string, age int32) *record.Record {
	r := record.NewRecord(s)
	r.Set(0, record.Int64Value(id))
	r.Set(1, record.StringValue(name))
	r.Set(2, record.Int32Value(age))
	return r
}

func TestInsertGet(t *testing.T) {
	hf, cleanup := setupHeapFile(t)
	defer cleanup()
	rec := makeRecord(hf.Schema(), 1, "Alice", 30)
	rid, err := hf.Insert(rec)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := hf.Get(rid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	v, _ := got.Get(0)
	id, _ := v.AsInt64()
	if id != 1 {
		t.Errorf("id: want 1, got %d", id)
	}
}

func TestUpdate(t *testing.T) {
	hf, cleanup := setupHeapFile(t)
	defer cleanup()
	rid, _ := hf.Insert(makeRecord(hf.Schema(), 1, "Bob", 25))
	hf.Update(rid, makeRecord(hf.Schema(), 1, "Bobby", 26))
	got, _ := hf.Get(rid)
	v, _ := got.Get(1)
	name, _ := v.AsString()
	if name != "Bobby" {
		t.Errorf("name: want Bobby, got %s", name)
	}
}

func TestDelete(t *testing.T) {
	hf, cleanup := setupHeapFile(t)
	defer cleanup()
	rid, _ := hf.Insert(makeRecord(hf.Schema(), 1, "Charlie", 35))
	hf.Delete(rid)
	_, err := hf.Get(rid)
	if err != ErrRecordNotFound {
		t.Error("expected ErrRecordNotFound")
	}
}

func TestMultipleInserts(t *testing.T) {
	hf, cleanup := setupHeapFile(t)
	defer cleanup()
	rids := make([]record.RecordID, 100)
	for i := 0; i < 100; i++ {
		rid, _ := hf.Insert(makeRecord(hf.Schema(), int64(i), "User", int32(i)))
		rids[i] = rid
	}
	for i, rid := range rids {
		rec, _ := hf.Get(rid)
		v, _ := rec.Get(0)
		id, _ := v.AsInt64()
		if id != int64(i) {
			t.Errorf("record %d: want %d, got %d", i, i, id)
		}
	}
}

func TestIterator(t *testing.T) {
	hf, cleanup := setupHeapFile(t)
	defer cleanup()
	for i := 0; i < 10; i++ {
		hf.Insert(makeRecord(hf.Schema(), int64(i), "User", int32(i)))
	}
	iter := hf.NewIterator()
	defer iter.Close()
	count := 0
	for {
		rec, _, err := iter.Next()
		if err != nil {
			t.Fatalf("iter: %v", err)
		}
		if rec == nil {
			break
		}
		count++
	}
	if count != 10 {
		t.Errorf("count: want 10, got %d", count)
	}
}

func BenchmarkInsert(b *testing.B) {
	tmpDir := b.TempDir()
	dm, _ := file.NewDiskManager(filepath.Join(tmpDir, "b.db"), file.DefaultConfig())
	bp := buffer.NewBufferPool(dm, buffer.Config{PoolSize: 1000})
	defer dm.Close()
	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt64},
		{Name: "name", Type: record.FieldTypeString},
	})
	hf, _ := NewHeapFile(HeapFileConfig{TableID: 1, Schema: schema, BufferPool: bp})
	rec := record.NewRecord(schema)
	rec.Set(0, record.Int64Value(1))
	rec.Set(1, record.StringValue("test"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hf.Insert(rec)
	}
}

func BenchmarkGet(b *testing.B) {
	tmpDir := b.TempDir()
	dm, _ := file.NewDiskManager(filepath.Join(tmpDir, "b.db"), file.DefaultConfig())
	bp := buffer.NewBufferPool(dm, buffer.Config{PoolSize: 1000})
	defer dm.Close()
	schema := record.NewSchema([]record.Field{
		{Name: "id", Type: record.FieldTypeInt64},
		{Name: "name", Type: record.FieldTypeString},
	})
	hf, _ := NewHeapFile(HeapFileConfig{TableID: 1, Schema: schema, BufferPool: bp})
	rec := record.NewRecord(schema)
	rec.Set(0, record.Int64Value(1))
	rec.Set(1, record.StringValue("test"))
	rids := make([]record.RecordID, 1000)
	for i := 0; i < 1000; i++ {
		rid, _ := hf.Insert(rec)
		rids[i] = rid
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hf.Get(rids[i%1000])
	}
}
