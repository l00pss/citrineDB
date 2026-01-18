package index

import (
	"github.com/l00pss/citrinedb/storage/record"
	"github.com/l00pss/treego/bplustree"
)

type IndexID uint32

type IndexEntry struct {
	Key      []byte
	RecordID record.RecordID
}

type BTreeIndex struct {
	id     IndexID
	name   string
	tree   *bplustree.BPlusTree[string, record.RecordID]
	unique bool
}

type IndexConfig struct {
	ID     IndexID
	Name   string
	Unique bool
	Degree int
}

func NewBTreeIndex(config IndexConfig) *BTreeIndex {
	degree := config.Degree
	if degree < 3 {
		degree = 50
	}
	return &BTreeIndex{
		id:     config.ID,
		name:   config.Name,
		tree:   bplustree.New[string, record.RecordID](degree),
		unique: config.Unique,
	}
}

func (idx *BTreeIndex) ID() IndexID    { return idx.id }
func (idx *BTreeIndex) Name() string   { return idx.name }
func (idx *BTreeIndex) IsUnique() bool { return idx.unique }
func (idx *BTreeIndex) Len() int       { return idx.tree.Len() }

func (idx *BTreeIndex) Insert(key string, rid record.RecordID) error {
	if idx.unique {
		if _, found := idx.tree.Search(key); found {
			return ErrDuplicateKey
		}
	}
	idx.tree.Insert(key, rid)
	return nil
}

func (idx *BTreeIndex) Search(key string) (record.RecordID, bool) {
	return idx.tree.Search(key)
}

func (idx *BTreeIndex) Delete(key string) bool {
	return idx.tree.Delete(key)
}

func (idx *BTreeIndex) Range(start, end string) []IndexEntry {
	entries := idx.tree.Range(start, end)
	result := make([]IndexEntry, len(entries))
	for i, e := range entries {
		result[i] = IndexEntry{Key: []byte(e.Key), RecordID: e.Value}
	}
	return result
}

func (idx *BTreeIndex) All() []IndexEntry {
	entries := idx.tree.All()
	if entries == nil {
		return nil
	}
	result := make([]IndexEntry, len(entries))
	for i, e := range entries {
		result[i] = IndexEntry{Key: []byte(e.Key), RecordID: e.Value}
	}
	return result
}
