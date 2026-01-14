package buffer

import (
	"errors"
	"sync"

	"github.com/l00pss/citrinedb/storage/file"
	"github.com/l00pss/citrinedb/storage/page"
)

var (
	ErrBufferPoolFull = errors.New("buffer: pool is full, no unpinned pages")
	ErrPageNotFound   = errors.New("buffer: page not found in pool")
)

type BufferPool struct {
	mu          sync.Mutex
	diskManager *file.DiskManager
	poolSize    int
	frames      []*Frame
	pageTable   map[page.PageID]int
	freeList    []int
	lruList     *lruList
}

type Frame struct {
	page     *page.Page
	pageID   page.PageID
	pinCount int
	dirty    bool
	valid    bool
}

type Config struct {
	PoolSize int
}

func DefaultConfig() Config {
	return Config{PoolSize: 1024}
}

func NewBufferPool(dm *file.DiskManager, config Config) *BufferPool {
	if config.PoolSize <= 0 {
		config.PoolSize = 1024
	}

	frames := make([]*Frame, config.PoolSize)
	freeList := make([]int, config.PoolSize)

	for i := 0; i < config.PoolSize; i++ {
		frames[i] = &Frame{}
		freeList[i] = i
	}

	return &BufferPool{
		diskManager: dm,
		poolSize:    config.PoolSize,
		frames:      frames,
		pageTable:   make(map[page.PageID]int),
		freeList:    freeList,
		lruList:     newLRUList(),
	}
}

func (bp *BufferPool) FetchPage(pageID page.PageID) (*page.Page, error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if frameIdx, ok := bp.pageTable[pageID]; ok {
		frame := bp.frames[frameIdx]
		frame.pinCount++
		bp.lruList.remove(frameIdx)
		return frame.page, nil
	}

	frameIdx, err := bp.getFrame()
	if err != nil {
		return nil, err
	}

	p, err := bp.diskManager.ReadPage(pageID)
	if err != nil {
		bp.freeList = append(bp.freeList, frameIdx)
		return nil, err
	}

	frame := bp.frames[frameIdx]
	frame.page = p
	frame.pageID = pageID
	frame.pinCount = 1
	frame.dirty = false
	frame.valid = true

	bp.pageTable[pageID] = frameIdx

	return p, nil
}

func (bp *BufferPool) NewPage(pageType page.PageType) (*page.Page, error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	pageID, err := bp.diskManager.AllocatePage()
	if err != nil {
		return nil, err
	}

	frameIdx, err := bp.getFrame()
	if err != nil {
		return nil, err
	}

	p := page.NewPage(pageID, pageType)

	frame := bp.frames[frameIdx]
	frame.page = p
	frame.pageID = pageID
	frame.pinCount = 1
	frame.dirty = true
	frame.valid = true

	bp.pageTable[pageID] = frameIdx

	return p, nil
}

func (bp *BufferPool) UnpinPage(pageID page.PageID, isDirty bool) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	frameIdx, ok := bp.pageTable[pageID]
	if !ok {
		return ErrPageNotFound
	}

	frame := bp.frames[frameIdx]

	if frame.pinCount <= 0 {
		return nil
	}

	frame.pinCount--

	if isDirty {
		frame.dirty = true
	}

	if frame.pinCount == 0 {
		bp.lruList.add(frameIdx)
	}

	return nil
}

func (bp *BufferPool) FlushPage(pageID page.PageID) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	frameIdx, ok := bp.pageTable[pageID]
	if !ok {
		return ErrPageNotFound
	}

	frame := bp.frames[frameIdx]

	if frame.dirty {
		if err := bp.diskManager.WritePage(frame.page); err != nil {
			return err
		}
		frame.dirty = false
	}

	return nil
}

func (bp *BufferPool) FlushAll() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for _, frame := range bp.frames {
		if frame.valid && frame.dirty {
			if err := bp.diskManager.WritePage(frame.page); err != nil {
				return err
			}
			frame.dirty = false
		}
	}

	return nil
}

func (bp *BufferPool) DeletePage(pageID page.PageID) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	frameIdx, ok := bp.pageTable[pageID]
	if !ok {
		return nil
	}

	frame := bp.frames[frameIdx]

	if frame.pinCount > 0 {
		return errors.New("buffer: cannot delete pinned page")
	}

	bp.lruList.remove(frameIdx)

	frame.page = nil
	frame.pageID = page.InvalidPageID
	frame.pinCount = 0
	frame.dirty = false
	frame.valid = false

	delete(bp.pageTable, pageID)
	bp.freeList = append(bp.freeList, frameIdx)

	return nil
}

func (bp *BufferPool) getFrame() (int, error) {
	if len(bp.freeList) > 0 {
		frameIdx := bp.freeList[len(bp.freeList)-1]
		bp.freeList = bp.freeList[:len(bp.freeList)-1]
		return frameIdx, nil
	}

	frameIdx, ok := bp.lruList.pop()
	if !ok {
		return -1, ErrBufferPoolFull
	}

	frame := bp.frames[frameIdx]

	if frame.dirty {
		if err := bp.diskManager.WritePage(frame.page); err != nil {
			bp.lruList.add(frameIdx)
			return -1, err
		}
	}

	delete(bp.pageTable, frame.pageID)

	frame.page = nil
	frame.pageID = page.InvalidPageID
	frame.pinCount = 0
	frame.dirty = false
	frame.valid = false

	return frameIdx, nil
}

func (bp *BufferPool) Size() int {
	return bp.poolSize
}

func (bp *BufferPool) Stats() Stats {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	stats := Stats{
		PoolSize:  bp.poolSize,
		FreeCount: len(bp.freeList),
	}

	for _, frame := range bp.frames {
		if frame.valid {
			stats.UsedCount++
			if frame.pinCount > 0 {
				stats.PinnedCount++
			}
			if frame.dirty {
				stats.DirtyCount++
			}
		}
	}

	return stats
}

type Stats struct {
	PoolSize    int
	UsedCount   int
	FreeCount   int
	PinnedCount int
	DirtyCount  int
}
