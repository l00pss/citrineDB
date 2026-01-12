package page

import "encoding/binary"

type Iterator struct {
	page       *Page
	currentIdx SlotID
	slotCount  uint16
}

func NewIterator(p *Page) *Iterator {
	p.mu.RLock()
	slotCount := binary.LittleEndian.Uint16(p.data[offsetSlotCount:])
	p.mu.RUnlock()
	return &Iterator{page: p, currentIdx: 0, slotCount: slotCount}
}

func (it *Iterator) Next() (SlotID, []byte, bool) {
	it.page.mu.RLock()
	defer it.page.mu.RUnlock()
	for it.currentIdx < SlotID(it.slotCount) {
		slotID := it.currentIdx
		it.currentIdx++
		offset, length, err := it.page.getSlot(slotID)
		if err == nil {
			record := make([]byte, length)
			copy(record, it.page.data[offset:offset+length])
			return slotID, record, true
		}
	}
	return 0, nil, false
}

func (it *Iterator) Reset() {
	it.currentIdx = 0
}

func (p *Page) ForEach(fn func(slotID SlotID, record []byte) bool) {
	it := NewIterator(p)
	for {
		slotID, record, ok := it.Next()
		if !ok {
			break
		}
		if !fn(slotID, record) {
			break
		}
	}
}
