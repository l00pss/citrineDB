package page

import (
	"bytes"
	"testing"
)

func TestNewPage(t *testing.T) {
	p := NewPage(1, PageTypeData)

	if p.ID() != 1 {
		t.Errorf("expected page ID 1, got %d", p.ID())
	}

	if p.Type() != PageTypeData {
		t.Errorf("expected page type %d, got %d", PageTypeData, p.Type())
	}

	if p.SlotCount() != 0 {
		t.Errorf("expected slot count 0, got %d", p.SlotCount())
	}

	expectedFreeSpace := uint16(DefaultPageSize - HeaderSize)
	if p.FreeSpace() != expectedFreeSpace {
		t.Errorf("expected free space %d, got %d", expectedFreeSpace, p.FreeSpace())
	}

	if !p.IsDirty() {
		t.Error("new page should be dirty")
	}
}

func TestPageInsertAndGet(t *testing.T) {
	p := NewPage(1, PageTypeData)

	record1 := []byte("Hello, World!")
	slotID1, err := p.Insert(record1)
	if err != nil {
		t.Fatalf("failed to insert record: %v", err)
	}
	if slotID1 != 0 {
		t.Errorf("expected slot ID 0, got %d", slotID1)
	}

	record2 := []byte("Second record here")
	slotID2, err := p.Insert(record2)
	if err != nil {
		t.Fatalf("failed to insert record: %v", err)
	}
	if slotID2 != 1 {
		t.Errorf("expected slot ID 1, got %d", slotID2)
	}

	if p.SlotCount() != 2 {
		t.Errorf("expected slot count 2, got %d", p.SlotCount())
	}

	retrieved1, err := p.Get(slotID1)
	if err != nil {
		t.Fatalf("failed to get record: %v", err)
	}
	if !bytes.Equal(retrieved1, record1) {
		t.Errorf("expected %s, got %s", record1, retrieved1)
	}

	retrieved2, err := p.Get(slotID2)
	if err != nil {
		t.Fatalf("failed to get record: %v", err)
	}
	if !bytes.Equal(retrieved2, record2) {
		t.Errorf("expected %s, got %s", record2, retrieved2)
	}
}

func TestPageUpdate(t *testing.T) {
	p := NewPage(1, PageTypeData)

	original := []byte("Original data")
	slotID, err := p.Insert(original)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	updated := []byte("Updated! data")
	err = p.Update(slotID, updated)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	retrieved, err := p.Get(slotID)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if !bytes.Equal(retrieved, updated) {
		t.Errorf("expected %s, got %s", updated, retrieved)
	}

	newData := []byte("Completely different and longer data")
	err = p.Update(slotID, newData)
	if err != nil {
		t.Fatalf("failed to update with different size: %v", err)
	}

	retrieved, err = p.Get(slotID)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if !bytes.Equal(retrieved, newData) {
		t.Errorf("expected %s, got %s", newData, retrieved)
	}
}

func TestPageDelete(t *testing.T) {
	p := NewPage(1, PageTypeData)

	record := []byte("To be deleted")
	slotID, err := p.Insert(record)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	err = p.Delete(slotID)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, err = p.Get(slotID)
	if err != ErrSlotDeleted {
		t.Errorf("expected ErrSlotDeleted, got %v", err)
	}
}

func TestPageFull(t *testing.T) {
	p := NewPage(1, PageTypeData)

	largeRecord := make([]byte, 1000)
	for i := range largeRecord {
		largeRecord[i] = byte(i % 256)
	}

	insertCount := 0
	for {
		_, err := p.Insert(largeRecord)
		if err == ErrPageFull {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		insertCount++
		if insertCount > 10 {
			break
		}
	}

	if insertCount == 0 {
		t.Error("should have been able to insert at least one record")
	}
}

func TestPageCompact(t *testing.T) {
	p := NewPage(1, PageTypeData)

	records := [][]byte{
		[]byte("Record 0"),
		[]byte("Record 1 - longer"),
		[]byte("Record 2"),
		[]byte("Record 3 - even longer record"),
	}

	slotIDs := make([]SlotID, len(records))
	for i, rec := range records {
		var err error
		slotIDs[i], err = p.Insert(rec)
		if err != nil {
			t.Fatalf("failed to insert record %d: %v", i, err)
		}
	}

	p.Delete(slotIDs[1])
	p.Delete(slotIDs[2])

	// Compact reorganizes the page
	p.Compact()

	// Remaining records should still be accessible
	retrieved0, err := p.Get(slotIDs[0])
	if err != nil {
		t.Fatalf("failed to get record 0: %v", err)
	}
	if !bytes.Equal(retrieved0, records[0]) {
		t.Errorf("record 0 mismatch")
	}

	retrieved3, err := p.Get(slotIDs[3])
	if err != nil {
		t.Fatalf("failed to get record 3: %v", err)
	}
	if !bytes.Equal(retrieved3, records[3]) {
		t.Errorf("record 3 mismatch")
	}

	// Deleted slots should still be deleted
	_, err = p.Get(slotIDs[1])
	if err != ErrSlotDeleted {
		t.Errorf("expected ErrSlotDeleted for slot 1, got %v", err)
	}

	_, err = p.Get(slotIDs[2])
	if err != ErrSlotDeleted {
		t.Errorf("expected ErrSlotDeleted for slot 2, got %v", err)
	}
}

func TestPageFromBytes(t *testing.T) {
	p1 := NewPage(42, PageTypeData)
	record := []byte("Persistent data")
	slotID, _ := p1.Insert(record)
	p1.SetLSN(12345)

	data := p1.ToBytes()

	p2, err := FromBytes(data)
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	if p2.ID() != 42 {
		t.Errorf("expected page ID 42, got %d", p2.ID())
	}

	if p2.Type() != PageTypeData {
		t.Errorf("expected page type %d, got %d", PageTypeData, p2.Type())
	}

	if p2.LSN() != 12345 {
		t.Errorf("expected LSN 12345, got %d", p2.LSN())
	}

	retrieved, err := p2.Get(slotID)
	if err != nil {
		t.Fatalf("failed to get record: %v", err)
	}
	if !bytes.Equal(retrieved, record) {
		t.Errorf("expected %s, got %s", record, retrieved)
	}
}

func TestPagePinning(t *testing.T) {
	p := NewPage(1, PageTypeData)

	if p.PinCount() != 0 {
		t.Errorf("expected initial pin count 0, got %d", p.PinCount())
	}

	p.Pin()
	p.Pin()

	if p.PinCount() != 2 {
		t.Errorf("expected pin count 2, got %d", p.PinCount())
	}

	p.Unpin()

	if p.PinCount() != 1 {
		t.Errorf("expected pin count 1, got %d", p.PinCount())
	}

	p.Unpin()
	p.Unpin()

	if p.PinCount() != 0 {
		t.Errorf("expected pin count 0, got %d", p.PinCount())
	}
}

func TestInvalidSlot(t *testing.T) {
	p := NewPage(1, PageTypeData)

	_, err := p.Get(SlotID(0))
	if err != ErrInvalidSlot {
		t.Errorf("expected ErrInvalidSlot, got %v", err)
	}

	err = p.Update(SlotID(0), []byte("data"))
	if err != ErrInvalidSlot {
		t.Errorf("expected ErrInvalidSlot, got %v", err)
	}

	err = p.Delete(SlotID(0))
	if err != ErrInvalidSlot {
		t.Errorf("expected ErrInvalidSlot, got %v", err)
	}
}

func BenchmarkPageInsert(b *testing.B) {
	p := NewPageWithSize(1, PageTypeData, 64*1024)
	record := []byte("benchmark record data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Insert(record)
		if err == ErrPageFull {
			p = NewPageWithSize(1, PageTypeData, 64*1024)
		}
	}
}

func BenchmarkPageGet(b *testing.B) {
	p := NewPage(1, PageTypeData)
	record := []byte("benchmark record data")
	slotID, _ := p.Insert(record)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Get(slotID)
	}
}
