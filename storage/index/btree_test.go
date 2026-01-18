package index

import (
	"fmt"
	"testing"

	"github.com/l00pss/citrinedb/storage/record"
)

func TestNewBTreeIndex(t *testing.T) {
	idx := NewBTreeIndex(IndexConfig{ID: 1, Name: "pk_users", Unique: true, Degree: 50})
	if idx.ID() != 1 {
		t.Errorf("ID: want 1, got %d", idx.ID())
	}
	if idx.Name() != "pk_users" {
		t.Errorf("Name: want pk_users, got %s", idx.Name())
	}
	if !idx.IsUnique() {
		t.Error("IsUnique: want true")
	}
	if idx.Len() != 0 {
		t.Errorf("Len: want 0, got %d", idx.Len())
	}
}

func TestInsertAndSearch(t *testing.T) {
	idx := NewBTreeIndex(IndexConfig{ID: 1, Name: "idx_name", Unique: false})
	rid1 := record.NewRecordID(1, 0)
	rid2 := record.NewRecordID(1, 1)
	rid3 := record.NewRecordID(2, 0)

	idx.Insert("alice", rid1)
	idx.Insert("bob", rid2)
	idx.Insert("charlie", rid3)

	if idx.Len() != 3 {
		t.Errorf("Len: want 3, got %d", idx.Len())
	}

	if rid, found := idx.Search("bob"); !found || rid != rid2 {
		t.Errorf("Search bob: want %v, got %v, found=%v", rid2, rid, found)
	}

	if _, found := idx.Search("dave"); found {
		t.Error("Search dave: should not be found")
	}
}

func TestUniqueConstraint(t *testing.T) {
	idx := NewBTreeIndex(IndexConfig{ID: 1, Name: "pk", Unique: true})
	rid1 := record.NewRecordID(1, 0)
	rid2 := record.NewRecordID(1, 1)

	if err := idx.Insert("key1", rid1); err != nil {
		t.Errorf("First insert should succeed: %v", err)
	}

	if err := idx.Insert("key1", rid2); err != ErrDuplicateKey {
		t.Errorf("Duplicate insert should fail with ErrDuplicateKey, got %v", err)
	}

	if idx.Len() != 1 {
		t.Errorf("Len after duplicate: want 1, got %d", idx.Len())
	}
}

func TestDelete(t *testing.T) {
	idx := NewBTreeIndex(IndexConfig{ID: 1, Name: "idx"})
	rid := record.NewRecordID(1, 0)
	idx.Insert("key", rid)

	if !idx.Delete("key") {
		t.Error("Delete existing key should return true")
	}

	if idx.Len() != 0 {
		t.Errorf("Len after delete: want 0, got %d", idx.Len())
	}

	if idx.Delete("nonexistent") {
		t.Error("Delete non-existent key should return false")
	}
}

func TestRange(t *testing.T) {
	idx := NewBTreeIndex(IndexConfig{ID: 1, Name: "idx"})
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%02d", i)
		rid := record.NewRecordID(uint32(i), 0)
		idx.Insert(key, rid)
	}

	entries := idx.Range("key03", "key07")
	if len(entries) != 5 {
		t.Errorf("Range: want 5 entries, got %d", len(entries))
	}

	for i, e := range entries {
		expectedKey := fmt.Sprintf("key%02d", i+3)
		if string(e.Key) != expectedKey {
			t.Errorf("Range entry %d: want %s, got %s", i, expectedKey, e.Key)
		}
	}
}

func TestAll(t *testing.T) {
	idx := NewBTreeIndex(IndexConfig{ID: 1, Name: "idx"})
	if idx.All() != nil {
		t.Error("All on empty index should return nil")
	}

	for i := 0; i < 5; i++ {
		idx.Insert(fmt.Sprintf("key%d", i), record.NewRecordID(uint32(i), 0))
	}

	entries := idx.All()
	if len(entries) != 5 {
		t.Errorf("All: want 5 entries, got %d", len(entries))
	}
}

func BenchmarkInsert(b *testing.B) {
	idx := NewBTreeIndex(IndexConfig{ID: 1, Name: "bench", Degree: 50})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%010d", i)
		rid := record.NewRecordID(uint32(i), 0)
		idx.Insert(key, rid)
	}
}

func BenchmarkSearch(b *testing.B) {
	idx := NewBTreeIndex(IndexConfig{ID: 1, Name: "bench", Degree: 50})
	for i := 0; i < 100000; i++ {
		idx.Insert(fmt.Sprintf("key%010d", i), record.NewRecordID(uint32(i), 0))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(fmt.Sprintf("key%010d", i%100000))
	}
}
