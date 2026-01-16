package table

import (
	"errors"
	"sync"

	"github.com/l00pss/citrinedb/storage/buffer"
	"github.com/l00pss/citrinedb/storage/page"
	"github.com/l00pss/citrinedb/storage/record"
)

var (
	ErrRecordNotFound = errors.New("table: record not found")
	ErrRecordTooLarge = errors.New("table: record too large for page")
)

type TableID uint32

type HeapFile struct {
	mu          sync.RWMutex
	tableID     TableID
	schema      *record.Schema
	bufferPool  *buffer.BufferPool
	firstPageID page.PageID
	lastPageID  page.PageID
	pageCount   uint32
}

type HeapFileConfig struct {
	TableID    TableID
	Schema     *record.Schema
	BufferPool *buffer.BufferPool
}

func NewHeapFile(config HeapFileConfig) (*HeapFile, error) {
	hf := &HeapFile{
		tableID:    config.TableID,
		schema:     config.Schema,
		bufferPool: config.BufferPool,
	}
	p, err := hf.bufferPool.NewPage(page.PageTypeData)
	if err != nil {
		return nil, err
	}
	hf.firstPageID = p.ID()
	hf.lastPageID = p.ID()
	hf.pageCount = 1
	hf.bufferPool.UnpinPage(p.ID(), true)
	return hf, nil
}

func (hf *HeapFile) Insert(rec *record.Record) (record.RecordID, error) {
	hf.mu.Lock()
	defer hf.mu.Unlock()
	data := rec.Serialize()
	if len(data) > page.MaxRecordSize {
		return record.RecordID{}, ErrRecordTooLarge
	}
	p, err := hf.bufferPool.FetchPage(hf.lastPageID)
	if err != nil {
		return record.RecordID{}, err
	}
	slotID, err := p.Insert(data)
	if err == nil {
		hf.bufferPool.UnpinPage(hf.lastPageID, true)
		return record.NewRecordID(uint32(hf.lastPageID), uint16(slotID)), nil
	}
	hf.bufferPool.UnpinPage(hf.lastPageID, false)
	newPage, err := hf.bufferPool.NewPage(page.PageTypeData)
	if err != nil {
		return record.RecordID{}, err
	}
	oldLast, _ := hf.bufferPool.FetchPage(hf.lastPageID)
	oldLast.SetNextPageID(newPage.ID())
	newPage.SetPrevPageID(hf.lastPageID)
	hf.bufferPool.UnpinPage(hf.lastPageID, true)
	hf.lastPageID = newPage.ID()
	hf.pageCount++
	slotID, err = newPage.Insert(data)
	if err != nil {
		hf.bufferPool.UnpinPage(newPage.ID(), true)
		return record.RecordID{}, err
	}
	hf.bufferPool.UnpinPage(newPage.ID(), true)
	return record.NewRecordID(uint32(newPage.ID()), uint16(slotID)), nil
}

func (hf *HeapFile) Get(rid record.RecordID) (*record.Record, error) {
	hf.mu.RLock()
	defer hf.mu.RUnlock()
	p, err := hf.bufferPool.FetchPage(page.PageID(rid.PageID))
	if err != nil {
		return nil, err
	}
	defer hf.bufferPool.UnpinPage(page.PageID(rid.PageID), false)
	data, err := p.Get(page.SlotID(rid.SlotID))
	if err != nil {
		return nil, ErrRecordNotFound
	}
	return record.DeserializeRecord(hf.schema, data)
}

func (hf *HeapFile) Update(rid record.RecordID, rec *record.Record) error {
	hf.mu.Lock()
	defer hf.mu.Unlock()
	data := rec.Serialize()
	p, err := hf.bufferPool.FetchPage(page.PageID(rid.PageID))
	if err != nil {
		return err
	}
	defer hf.bufferPool.UnpinPage(page.PageID(rid.PageID), true)
	return p.Update(page.SlotID(rid.SlotID), data)
}

func (hf *HeapFile) Delete(rid record.RecordID) error {
	hf.mu.Lock()
	defer hf.mu.Unlock()
	p, err := hf.bufferPool.FetchPage(page.PageID(rid.PageID))
	if err != nil {
		return err
	}
	defer hf.bufferPool.UnpinPage(page.PageID(rid.PageID), true)
	return p.Delete(page.SlotID(rid.SlotID))
}

func (hf *HeapFile) TableID() TableID         { return hf.tableID }
func (hf *HeapFile) Schema() *record.Schema   { return hf.schema }
func (hf *HeapFile) FirstPageID() page.PageID { return hf.firstPageID }
func (hf *HeapFile) PageCount() uint32        { return hf.pageCount }

type HeapIterator struct {
	heapFile      *HeapFile
	currentPageID page.PageID
	currentPage   *page.Page
	pageIter      *page.Iterator
	finished      bool
}

func (hf *HeapFile) NewIterator() *HeapIterator {
	return &HeapIterator{heapFile: hf, currentPageID: hf.firstPageID}
}

func (it *HeapIterator) Next() (*record.Record, record.RecordID, error) {
	if it.finished {
		return nil, record.RecordID{}, nil
	}
	for {
		if it.currentPage == nil {
			if it.currentPageID == page.InvalidPageID {
				it.finished = true
				return nil, record.RecordID{}, nil
			}
			p, err := it.heapFile.bufferPool.FetchPage(it.currentPageID)
			if err != nil {
				return nil, record.RecordID{}, err
			}
			it.currentPage = p
			it.pageIter = p.NewIterator()
		}
		slotID, data, ok := it.pageIter.Next()
		if ok {
			rec, err := record.DeserializeRecord(it.heapFile.schema, data)
			if err != nil {
				continue
			}
			rid := record.NewRecordID(uint32(it.currentPageID), uint16(slotID))
			return rec, rid, nil
		}
		nextPageID := it.currentPage.NextPageID()
		it.heapFile.bufferPool.UnpinPage(it.currentPageID, false)
		it.currentPage = nil
		it.pageIter = nil
		if nextPageID == page.InvalidPageID {
			it.finished = true
			return nil, record.RecordID{}, nil
		}
		it.currentPageID = nextPageID
	}
}

func (it *HeapIterator) Close() {
	if it.currentPage != nil {
		it.heapFile.bufferPool.UnpinPage(it.currentPageID, false)
		it.currentPage = nil
	}
	it.finished = true
}

func (it *HeapIterator) Reset() {
	if it.currentPage != nil {
		it.heapFile.bufferPool.UnpinPage(it.currentPageID, false)
	}
	it.currentPageID = it.heapFile.firstPageID
	it.currentPage = nil
	it.pageIter = nil
	it.finished = false
}
