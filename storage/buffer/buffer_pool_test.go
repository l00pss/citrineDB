package buffer

import (
	"path/filepath"
	"testing"

	"github.com/l00pss/citrinedb/storage/file"
	"github.com/l00pss/citrinedb/storage/page"
)

func setupBufferPool(t *testing.T, poolSize int) (*BufferPool, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := file.NewDiskManager(dbPath, file.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create disk manager: %v", err)
	}

	bp := NewBufferPool(dm, Config{PoolSize: poolSize})

	cleanup := func() {
		bp.FlushAll()
		dm.Close()
	}

	return bp, cleanup
}

func TestNewBufferPool(t *testing.T) {
	bp, cleanup := setupBufferPool(t, 10)
	defer cleanup()

	if bp.Size() != 10 {
		t.Errorf("expected pool size 10, got %d", bp.Size())
	}

	stats := bp.Stats()
	if stats.FreeCount != 10 {
		t.Errorf("expected 10 free frames, got %d", stats.FreeCount)
	}
}

func TestBufferPoolNewPage(t *testing.T) {
	bp, cleanup := setupBufferPool(t, 10)
	defer cleanup()

	p, err := bp.NewPage(page.PageTypeData)
	if err != nil {
		t.Fatalf("failed to create new page: %v", err)
	}

	if p.ID() != 0 {
		t.Errorf("expected page ID 0, got %d", p.ID())
	}

	stats := bp.Stats()
	if stats.UsedCount != 1 {
		t.Errorf("expected 1 used frame, got %d", stats.UsedCount)
	}
	if stats.PinnedCount != 1 {
		t.Errorf("expected 1 pinned frame, got %d", stats.PinnedCount)
	}
}

func TestBufferPoolFetchPage(t *testing.T) {
	bp, cleanup := setupBufferPool(t, 10)
	defer cleanup()

	p1, _ := bp.NewPage(page.PageTypeData)
	pageID := p1.ID()

	testData := []byte("Hello Buffer Pool!")
	slotID, _ := p1.Insert(testData)

	bp.UnpinPage(pageID, true)
	bp.FlushPage(pageID)
	bp.DeletePage(pageID)

	p2, err := bp.FetchPage(pageID)
	if err != nil {
		t.Fatalf("failed to fetch page: %v", err)
	}

	readData, err := p2.Get(slotID)
	if err != nil {
		t.Fatalf("failed to get data: %v", err)
	}

	if string(readData) != string(testData) {
		t.Errorf("data mismatch: expected %s, got %s", testData, readData)
	}
}

func TestBufferPoolUnpin(t *testing.T) {
	bp, cleanup := setupBufferPool(t, 10)
	defer cleanup()

	p, _ := bp.NewPage(page.PageTypeData)
	pageID := p.ID()

	stats := bp.Stats()
	if stats.PinnedCount != 1 {
		t.Errorf("expected 1 pinned, got %d", stats.PinnedCount)
	}

	bp.UnpinPage(pageID, false)

	stats = bp.Stats()
	if stats.PinnedCount != 0 {
		t.Errorf("expected 0 pinned, got %d", stats.PinnedCount)
	}
}

func TestBufferPoolLRUReplacement(t *testing.T) {
	bp, cleanup := setupBufferPool(t, 3)
	defer cleanup()

	p1, _ := bp.NewPage(page.PageTypeData)
	p2, _ := bp.NewPage(page.PageTypeData)
	p3, _ := bp.NewPage(page.PageTypeData)

	id1, id2, id3 := p1.ID(), p2.ID(), p3.ID()

	bp.UnpinPage(id1, true)
	bp.UnpinPage(id2, true)
	bp.UnpinPage(id3, true)

	p4, err := bp.NewPage(page.PageTypeData)
	if err != nil {
		t.Fatalf("failed to create page after eviction: %v", err)
	}

	bp.UnpinPage(p4.ID(), true)

	p1Again, err := bp.FetchPage(id1)
	if err != nil {
		t.Fatalf("failed to fetch evicted page: %v", err)
	}

	if p1Again.ID() != id1 {
		t.Errorf("expected page ID %d, got %d", id1, p1Again.ID())
	}
}

func TestBufferPoolFlushAll(t *testing.T) {
	bp, cleanup := setupBufferPool(t, 10)
	defer cleanup()

	for i := 0; i < 5; i++ {
		p, _ := bp.NewPage(page.PageTypeData)
		p.Insert([]byte("test data"))
		bp.UnpinPage(p.ID(), true)
	}

	stats := bp.Stats()
	if stats.DirtyCount != 5 {
		t.Errorf("expected 5 dirty pages, got %d", stats.DirtyCount)
	}

	bp.FlushAll()

	stats = bp.Stats()
	if stats.DirtyCount != 0 {
		t.Errorf("expected 0 dirty pages after flush, got %d", stats.DirtyCount)
	}
}

func TestBufferPoolFull(t *testing.T) {
	bp, cleanup := setupBufferPool(t, 3)
	defer cleanup()

	bp.NewPage(page.PageTypeData)
	bp.NewPage(page.PageTypeData)
	bp.NewPage(page.PageTypeData)

	_, err := bp.NewPage(page.PageTypeData)
	if err != ErrBufferPoolFull {
		t.Errorf("expected ErrBufferPoolFull, got %v", err)
	}
}

func TestBufferPoolCacheHit(t *testing.T) {
	bp, cleanup := setupBufferPool(t, 10)
	defer cleanup()

	p1, _ := bp.NewPage(page.PageTypeData)
	pageID := p1.ID()
	bp.UnpinPage(pageID, false)

	p2, err := bp.FetchPage(pageID)
	if err != nil {
		t.Fatalf("cache hit failed: %v", err)
	}

	if p1 != p2 {
		t.Error("cache hit should return same page object")
	}
}

func BenchmarkBufferPoolNewPage(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	dm, _ := file.NewDiskManager(dbPath, file.DefaultConfig())
	bp := NewBufferPool(dm, Config{PoolSize: 10000})
	defer dm.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, _ := bp.NewPage(page.PageTypeData)
		bp.UnpinPage(p.ID(), false)
	}
}

func BenchmarkBufferPoolFetch(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	dm, _ := file.NewDiskManager(dbPath, file.DefaultConfig())
	bp := NewBufferPool(dm, Config{PoolSize: 1000})
	defer dm.Close()

	pageIDs := make([]page.PageID, 100)
	for i := 0; i < 100; i++ {
		p, _ := bp.NewPage(page.PageTypeData)
		pageIDs[i] = p.ID()
		bp.UnpinPage(p.ID(), false)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pageID := pageIDs[i%100]
		p, _ := bp.FetchPage(pageID)
		bp.UnpinPage(p.ID(), false)
	}
}
