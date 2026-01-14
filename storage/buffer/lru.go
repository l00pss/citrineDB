package buffer

type lruList struct {
	head *lruNode
	tail *lruNode
	idx  map[int]*lruNode
}

type lruNode struct {
	frameIdx int
	prev     *lruNode
	next     *lruNode
}

func newLRUList() *lruList {
	return &lruList{
		idx: make(map[int]*lruNode),
	}
}

func (l *lruList) add(frameIdx int) {
	if _, exists := l.idx[frameIdx]; exists {
		return
	}

	node := &lruNode{frameIdx: frameIdx}
	l.idx[frameIdx] = node

	if l.tail == nil {
		l.head = node
		l.tail = node
	} else {
		node.prev = l.tail
		l.tail.next = node
		l.tail = node
	}
}

func (l *lruList) remove(frameIdx int) {
	node, exists := l.idx[frameIdx]
	if !exists {
		return
	}

	delete(l.idx, frameIdx)

	if node.prev != nil {
		node.prev.next = node.next
	} else {
		l.head = node.next
	}

	if node.next != nil {
		node.next.prev = node.prev
	} else {
		l.tail = node.prev
	}
}

func (l *lruList) pop() (int, bool) {
	if l.head == nil {
		return -1, false
	}

	node := l.head
	l.remove(node.frameIdx)
	return node.frameIdx, true
}

func (l *lruList) size() int {
	return len(l.idx)
}
