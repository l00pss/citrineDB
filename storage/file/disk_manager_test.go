package file

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/l00pss/citrinedb/storage/page"
)

func TestNewDiskManager(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDiskManager(dbPath, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	if dm.NumPages() != 0 {
		t.Errorf("expected 0 pages, got %d", dm.NumPages())
	}

	if dm.PageSize() != page.DefaultPageSize {
		t.Errorf("expected page size %d, got %d", page.DefaultPageSize, dm.PageSize())
	}
}

func TestDiskManagerReopenExisting(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm1, err := NewDiskManager(dbPath, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}

	dm1.AllocatePage()
	dm1.AllocatePage()
	dm1.Close()

	dm2, err := NewDiskManager(dbPath, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to reopen disk manager: %v", err)
	}
	defer dm2.Close()

	if dm2.NumPages() != 2 {
		t.Errorf("expected 2 pages after reopen, got %d", dm2.NumPages())
	}
}

func TestDiskManagerAllocatePage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDiskManager(dbPath, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	pageID1, err := dm.AllocatePage()
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}
	if pageID1 != 0 {
		t.Errorf("expected page ID 0, got %d", pageID1)
	}

	pageID2, err := dm.AllocatePage()
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}
	if pageID2 != 1 {
		t.Errorf("expected page ID 1, got %d", pageID2)
	}

	if dm.NumPages() != 2 {
		t.Errorf("expected 2 pages, got %d", dm.NumPages())
	}
}

func TestDiskManagerReadWritePage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDiskManager(dbPath, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	pageID, err := dm.AllocatePage()
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	p := page.NewPage(pageID, page.PageTypeData)
	testData := []byte("Hello, Disk Manager!")
	slotID, err := p.Insert(testData)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	if err := dm.WritePage(p); err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	readPage, err := dm.ReadPage(pageID)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}

	readData, err := readPage.Get(slotID)
	if err != nil {
		t.Fatalf("failed to get data: %v", err)
	}

	if !bytes.Equal(readData, testData) {
		t.Errorf("data mismatch: expected %s, got %s", testData, readData)
	}
}

func TestDiskManagerPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm1, err := NewDiskManager(dbPath, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}

	pageID, _ := dm1.AllocatePage()
	p := page.NewPage(pageID, page.PageTypeData)
	testData := []byte("Persistent data test")
	slotID, _ := p.Insert(testData)
	dm1.WritePage(p)
	dm1.Close()

	dm2, err := NewDiskManager(dbPath, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}
	defer dm2.Close()

	readPage, err := dm2.ReadPage(pageID)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}

	readData, err := readPage.Get(slotID)
	if err != nil {
		t.Fatalf("failed to get data: %v", err)
	}

	if !bytes.Equal(readData, testData) {
		t.Errorf("persistent data mismatch")
	}
}

func TestDiskManagerInvalidPageID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDiskManager(dbPath, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	_, err = dm.ReadPage(0)
	if err != ErrInvalidPageID {
		t.Errorf("expected ErrInvalidPageID, got %v", err)
	}

	dm.AllocatePage()

	_, err = dm.ReadPage(1)
	if err != ErrInvalidPageID {
		t.Errorf("expected ErrInvalidPageID, got %v", err)
	}
}

func TestDiskManagerFileSize(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDiskManager(dbPath, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}
	defer dm.Close()

	size, err := dm.FileSize()
	if err != nil {
		t.Fatalf("failed to get file size: %v", err)
	}
	if size != FileHeaderSize {
		t.Errorf("expected initial size %d, got %d", FileHeaderSize, size)
	}

	dm.AllocatePage()
	dm.AllocatePage()

	size, err = dm.FileSize()
	if err != nil {
		t.Fatalf("failed to get file size: %v", err)
	}
	expectedSize := int64(FileHeaderSize + 2*page.DefaultPageSize)
	if size != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, size)
	}
}

func TestDiskManagerInvalidMagic(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	f, _ := os.Create(dbPath)
	// Write exactly 64 bytes with invalid magic
	header := make([]byte, 64)
	copy(header, []byte("BAAD")) // Invalid magic number
	f.Write(header)
	f.Close()

	_, err := NewDiskManager(dbPath, DefaultConfig())
	if err != ErrInvalidMagic {
		t.Errorf("expected ErrInvalidMagic, got %v", err)
	}
}

func TestDiskManagerClosedOperations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, _ := NewDiskManager(dbPath, DefaultConfig())
	dm.AllocatePage()
	dm.Close()

	_, err := dm.ReadPage(0)
	if err != ErrFileClosed {
		t.Errorf("expected ErrFileClosed on read, got %v", err)
	}

	_, err = dm.AllocatePage()
	if err != ErrFileClosed {
		t.Errorf("expected ErrFileClosed on allocate, got %v", err)
	}
}

func BenchmarkDiskManagerWrite(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	dm, _ := NewDiskManager(dbPath, DefaultConfig())
	defer dm.Close()

	pageID, _ := dm.AllocatePage()
	p := page.NewPage(pageID, page.PageTypeData)
	p.Insert([]byte("benchmark data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dm.WritePage(p)
	}
}

func BenchmarkDiskManagerRead(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	dm, _ := NewDiskManager(dbPath, DefaultConfig())
	defer dm.Close()

	pageID, _ := dm.AllocatePage()
	p := page.NewPage(pageID, page.PageTypeData)
	p.Insert([]byte("benchmark data"))
	dm.WritePage(p)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dm.ReadPage(pageID)
	}
}
