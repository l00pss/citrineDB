package page

import (
	"encoding/binary"
	"errors"
	"sync"
)

const (
	DefaultPageSize = 4096
	HeaderSize      = 32
	SlotSize        = 4
	MaxRecordSize   = DefaultPageSize - HeaderSize - SlotSize
)

type PageID uint32

const InvalidPageID PageID = ^PageID(0)

type PageType uint8

const (
	PageTypeData PageType = iota + 1
	PageTypeIndex
	PageTypeOverflow
	PageTypeFree
	PageTypeMeta
)

type SlotID uint16

var (
	ErrPageFull       = errors.New("page: not enough space")
	ErrInvalidSlot    = errors.New("page: invalid slot id")
	ErrSlotDeleted    = errors.New("page: slot has been deleted")
	ErrRecordTooLarge = errors.New("page: record too large for page")
)

type Page struct {
	mu       sync.RWMutex
	id       PageID
	pageType PageType
	data     []byte
	dirty    bool
	pinCount int
}

const (
	offsetPageID       = 0
	offsetPageType     = 4
	offsetFlags        = 5
	offsetSlotCount    = 6
	offsetFreeSpacePtr = 8
	offsetFreeSpace    = 10
	offsetLSN          = 12
	offsetChecksum     = 20
	offsetNextPageID   = 24
	offsetPrevPageID   = 28
)

func NewPage(id PageID, pageType PageType) *Page {
	p := &Page{id: id, pageType: pageType, data: make([]byte, DefaultPageSize), dirty: true}
	p.initialize()
	return p
}

func NewPageWithSize(id PageID, pageType PageType, size int) *Page {
	p := &Page{id: id, pageType: pageType, data: make([]byte, size), dirty: true}
	p.initialize()
	return p
}

func FromBytes(data []byte) (*Page, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("page: data too small")
	}
	p := &Page{data: make([]byte, len(data)), dirty: false}
	copy(p.data, data)
	p.id = PageID(binary.LittleEndian.Uint32(p.data[offsetPageID:]))
	p.pageType = PageType(p.data[offsetPageType])
	return p, nil
}

func (p *Page) initialize() {
	binary.LittleEndian.PutUint32(p.data[offsetPageID:], uint32(p.id))
	p.data[offsetPageType] = byte(p.pageType)
	p.data[offsetFlags] = 0
	binary.LittleEndian.PutUint16(p.data[offsetSlotCount:], 0)
	binary.LittleEndian.PutUint16(p.data[offsetFreeSpacePtr:], uint16(len(p.data)))
	binary.LittleEndian.PutUint16(p.data[offsetFreeSpace:], uint16(len(p.data)-HeaderSize))
	binary.LittleEndian.PutUint64(p.data[offsetLSN:], 0)
	binary.LittleEndian.PutUint32(p.data[offsetChecksum:], 0)
	binary.LittleEndian.PutUint32(p.data[offsetNextPageID:], uint32(InvalidPageID))
	binary.LittleEndian.PutUint32(p.data[offsetPrevPageID:], uint32(InvalidPageID))
}

func (p *Page) ID() PageID      { return p.id }
func (p *Page) Type() PageType  { return p.pageType }
func (p *Page) IsDirty() bool   { p.mu.RLock(); defer p.mu.RUnlock(); return p.dirty }
func (p *Page) PinCount() int   { p.mu.RLock(); defer p.mu.RUnlock(); return p.pinCount }
func (p *Page) SetDirty(d bool) { p.mu.Lock(); defer p.mu.Unlock(); p.dirty = d }
func (p *Page) Pin()            { p.mu.Lock(); defer p.mu.Unlock(); p.pinCount++ }
func (p *Page) Unpin() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pinCount > 0 {
		p.pinCount--
	}
}

func (p *Page) SlotCount() uint16 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return binary.LittleEndian.Uint16(p.data[offsetSlotCount:])
}

func (p *Page) FreeSpace() uint16 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return binary.LittleEndian.Uint16(p.data[offsetFreeSpace:])
}

func (p *Page) LSN() uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return binary.LittleEndian.Uint64(p.data[offsetLSN:])
}

func (p *Page) SetLSN(lsn uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	binary.LittleEndian.PutUint64(p.data[offsetLSN:], lsn)
	p.dirty = true
}

func (p *Page) NextPageID() PageID {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return PageID(binary.LittleEndian.Uint32(p.data[offsetNextPageID:]))
}

func (p *Page) PrevPageID() PageID {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return PageID(binary.LittleEndian.Uint32(p.data[offsetPrevPageID:]))
}

func (p *Page) SetNextPageID(id PageID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	binary.LittleEndian.PutUint32(p.data[offsetNextPageID:], uint32(id))
	p.dirty = true
}

func (p *Page) SetPrevPageID(id PageID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	binary.LittleEndian.PutUint32(p.data[offsetPrevPageID:], uint32(id))
	p.dirty = true
}

func (p *Page) ToBytes() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]byte, len(p.data))
	copy(result, p.data)
	return result
}

func (p *Page) getSlot(slotID SlotID) (offset uint16, length uint16, err error) {
	slotCount := binary.LittleEndian.Uint16(p.data[offsetSlotCount:])
	if uint16(slotID) >= slotCount {
		return 0, 0, ErrInvalidSlot
	}
	slotOffset := HeaderSize + int(slotID)*SlotSize
	offset = binary.LittleEndian.Uint16(p.data[slotOffset:])
	length = binary.LittleEndian.Uint16(p.data[slotOffset+2:])
	if offset == 0 && length == 0 {
		return 0, 0, ErrSlotDeleted
	}
	return offset, length, nil
}

func (p *Page) setSlot(slotID SlotID, offset, length uint16) {
	slotOffset := HeaderSize + int(slotID)*SlotSize
	binary.LittleEndian.PutUint16(p.data[slotOffset:], offset)
	binary.LittleEndian.PutUint16(p.data[slotOffset+2:], length)
}

func (p *Page) Insert(record []byte) (SlotID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	recordLen := uint16(len(record))
	requiredSpace := recordLen + SlotSize
	freeSpace := binary.LittleEndian.Uint16(p.data[offsetFreeSpace:])
	if requiredSpace > freeSpace {
		return 0, ErrPageFull
	}
	if int(recordLen) > len(p.data)-HeaderSize-SlotSize {
		return 0, ErrRecordTooLarge
	}

	slotCount := binary.LittleEndian.Uint16(p.data[offsetSlotCount:])
	freeSpacePtr := binary.LittleEndian.Uint16(p.data[offsetFreeSpacePtr:])
	newRecordOffset := freeSpacePtr - recordLen
	copy(p.data[newRecordOffset:], record)

	newSlotID := SlotID(slotCount)
	p.setSlot(newSlotID, newRecordOffset, recordLen)

	binary.LittleEndian.PutUint16(p.data[offsetSlotCount:], slotCount+1)
	binary.LittleEndian.PutUint16(p.data[offsetFreeSpacePtr:], newRecordOffset)
	binary.LittleEndian.PutUint16(p.data[offsetFreeSpace:], freeSpace-requiredSpace)

	p.dirty = true
	return newSlotID, nil
}

func (p *Page) Get(slotID SlotID) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	offset, length, err := p.getSlot(slotID)
	if err != nil {
		return nil, err
	}
	record := make([]byte, length)
	copy(record, p.data[offset:offset+length])
	return record, nil
}

func (p *Page) Update(slotID SlotID, record []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	offset, oldLen, err := p.getSlot(slotID)
	if err != nil {
		return err
	}
	newLen := uint16(len(record))

	if newLen == oldLen {
		copy(p.data[offset:], record)
		p.dirty = true
		return nil
	}

	freeSpace := binary.LittleEndian.Uint16(p.data[offsetFreeSpace:])
	if newLen > oldLen {
		extraSpace := newLen - oldLen
		if extraSpace > freeSpace {
			return ErrPageFull
		}
	}

	p.setSlot(slotID, 0, 0)
	freeSpacePtr := binary.LittleEndian.Uint16(p.data[offsetFreeSpacePtr:])
	newOffset := freeSpacePtr - newLen
	copy(p.data[newOffset:], record)
	p.setSlot(slotID, newOffset, newLen)

	binary.LittleEndian.PutUint16(p.data[offsetFreeSpacePtr:], newOffset)
	newFreeSpace := freeSpace + oldLen - newLen
	binary.LittleEndian.PutUint16(p.data[offsetFreeSpace:], newFreeSpace)

	p.dirty = true
	return nil
}

func (p *Page) Delete(slotID SlotID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, length, err := p.getSlot(slotID)
	if err != nil {
		return err
	}
	p.setSlot(slotID, 0, 0)
	freeSpace := binary.LittleEndian.Uint16(p.data[offsetFreeSpace:])
	binary.LittleEndian.PutUint16(p.data[offsetFreeSpace:], freeSpace+length)
	p.dirty = true
	return nil
}

func (p *Page) Compact() {
	p.mu.Lock()
	defer p.mu.Unlock()

	slotCount := binary.LittleEndian.Uint16(p.data[offsetSlotCount:])
	if slotCount == 0 {
		return
	}

	type recordInfo struct {
		slotID SlotID
		data   []byte
	}
	records := make([]recordInfo, 0, slotCount)

	for i := SlotID(0); i < SlotID(slotCount); i++ {
		offset, length, err := p.getSlot(i)
		if err != nil {
			continue
		}
		data := make([]byte, length)
		copy(data, p.data[offset:offset+length])
		records = append(records, recordInfo{slotID: i, data: data})
	}

	freeSpacePtr := uint16(len(p.data))
	binary.LittleEndian.PutUint16(p.data[offsetFreeSpacePtr:], freeSpacePtr)

	for _, rec := range records {
		recLen := uint16(len(rec.data))
		freeSpacePtr -= recLen
		copy(p.data[freeSpacePtr:], rec.data)
		p.setSlot(rec.slotID, freeSpacePtr, recLen)
	}

	slotArrayEnd := HeaderSize + int(slotCount)*SlotSize
	freeSpace := freeSpacePtr - uint16(slotArrayEnd)
	binary.LittleEndian.PutUint16(p.data[offsetFreeSpacePtr:], freeSpacePtr)
	binary.LittleEndian.PutUint16(p.data[offsetFreeSpace:], freeSpace)
	p.dirty = true
}

func (p *Page) NewIterator() *Iterator {
	return NewIterator(p)
}
