package file

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/l00pss/citrinedb/storage/page"
)

type DiskManager struct {
	mu       sync.RWMutex
	file     *os.File
	filePath string
	pageSize int
	numPages uint32
	closed   bool
}

const (
	FileHeaderSize    = 64
	MagicNumber       = 0x43495452
	FileFormatVersion = 1
)

var (
	ErrFileNotOpen      = errors.New("file: database file not open")
	ErrFileClosed       = errors.New("file: database file is closed")
	ErrInvalidPageID    = errors.New("file: invalid page ID")
	ErrInvalidMagic     = errors.New("file: invalid magic number")
	ErrVersionMismatch  = errors.New("file: file format version mismatch")
	ErrPageSizeMismatch = errors.New("file: page size mismatch")
	ErrReadFailed       = errors.New("file: read operation failed")
	ErrWriteFailed      = errors.New("file: write operation failed")
)

type Config struct {
	PageSize int
}

func DefaultConfig() Config {
	return Config{PageSize: page.DefaultPageSize}
}

func NewDiskManager(filePath string, config Config) (*DiskManager, error) {
	if config.PageSize == 0 {
		config.PageSize = page.DefaultPageSize
	}

	dm := &DiskManager{
		filePath: filePath,
		pageSize: config.PageSize,
	}

	exists := true
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		exists = false
	}

	var err error
	if exists {
		err = dm.openExisting()
	} else {
		err = dm.createNew()
	}

	if err != nil {
		return nil, err
	}

	return dm, nil
}

func (dm *DiskManager) createNew() error {
	file, err := os.OpenFile(dm.filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("file: failed to create file: %w", err)
	}
	dm.file = file

	header := make([]byte, FileHeaderSize)
	binary.LittleEndian.PutUint32(header[0:], MagicNumber)
	binary.LittleEndian.PutUint32(header[4:], FileFormatVersion)
	binary.LittleEndian.PutUint32(header[8:], uint32(dm.pageSize))
	binary.LittleEndian.PutUint32(header[12:], 0)
	binary.LittleEndian.PutUint32(header[16:], 0)

	if _, err := dm.file.Write(header); err != nil {
		dm.file.Close()
		os.Remove(dm.filePath)
		return fmt.Errorf("file: failed to write header: %w", err)
	}

	if err := dm.file.Sync(); err != nil {
		dm.file.Close()
		os.Remove(dm.filePath)
		return fmt.Errorf("file: failed to sync header: %w", err)
	}

	dm.numPages = 0
	return nil
}

func (dm *DiskManager) openExisting() error {
	file, err := os.OpenFile(dm.filePath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("file: failed to open file: %w", err)
	}
	dm.file = file

	header := make([]byte, FileHeaderSize)
	if _, err := dm.file.ReadAt(header, 0); err != nil {
		dm.file.Close()
		return fmt.Errorf("file: failed to read header: %w", err)
	}

	magic := binary.LittleEndian.Uint32(header[0:])
	if magic != MagicNumber {
		dm.file.Close()
		return ErrInvalidMagic
	}

	version := binary.LittleEndian.Uint32(header[4:])
	if version != FileFormatVersion {
		dm.file.Close()
		return ErrVersionMismatch
	}

	filePageSize := int(binary.LittleEndian.Uint32(header[8:]))
	if filePageSize != dm.pageSize {
		dm.file.Close()
		return ErrPageSizeMismatch
	}

	dm.numPages = binary.LittleEndian.Uint32(header[12:])
	return nil
}

func (dm *DiskManager) ReadPage(pageID page.PageID) (*page.Page, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.closed {
		return nil, ErrFileClosed
	}

	if uint32(pageID) >= dm.numPages {
		return nil, ErrInvalidPageID
	}

	offset := dm.getPageOffset(pageID)
	data := make([]byte, dm.pageSize)

	n, err := dm.file.ReadAt(data, offset)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	if n != dm.pageSize {
		return nil, fmt.Errorf("%w: incomplete read", ErrReadFailed)
	}

	return page.FromBytes(data)
}

func (dm *DiskManager) WritePage(p *page.Page) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.closed {
		return ErrFileClosed
	}

	pageID := p.ID()
	if uint32(pageID) >= dm.numPages {
		return ErrInvalidPageID
	}

	offset := dm.getPageOffset(pageID)
	data := p.ToBytes()

	n, err := dm.file.WriteAt(data, offset)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}
	if n != len(data) {
		return fmt.Errorf("%w: incomplete write", ErrWriteFailed)
	}

	return nil
}

func (dm *DiskManager) AllocatePage() (page.PageID, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.closed {
		return page.InvalidPageID, ErrFileClosed
	}

	newPageID := page.PageID(dm.numPages)
	offset := dm.getPageOffset(newPageID)

	emptyPage := make([]byte, dm.pageSize)
	if _, err := dm.file.WriteAt(emptyPage, offset); err != nil {
		return page.InvalidPageID, fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}

	dm.numPages++

	if err := dm.updateHeader(); err != nil {
		return page.InvalidPageID, err
	}

	return newPageID, nil
}

func (dm *DiskManager) DeallocatePage(pageID page.PageID) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.closed {
		return ErrFileClosed
	}

	if uint32(pageID) >= dm.numPages {
		return ErrInvalidPageID
	}

	offset := dm.getPageOffset(pageID)
	emptyPage := make([]byte, dm.pageSize)

	if _, err := dm.file.WriteAt(emptyPage, offset); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}

	return nil
}

func (dm *DiskManager) getPageOffset(pageID page.PageID) int64 {
	return int64(FileHeaderSize) + int64(pageID)*int64(dm.pageSize)
}

func (dm *DiskManager) updateHeader() error {
	header := make([]byte, FileHeaderSize)
	binary.LittleEndian.PutUint32(header[0:], MagicNumber)
	binary.LittleEndian.PutUint32(header[4:], FileFormatVersion)
	binary.LittleEndian.PutUint32(header[8:], uint32(dm.pageSize))
	binary.LittleEndian.PutUint32(header[12:], dm.numPages)
	binary.LittleEndian.PutUint32(header[16:], 0)

	if _, err := dm.file.WriteAt(header, 0); err != nil {
		return fmt.Errorf("file: failed to update header: %w", err)
	}

	return nil
}

func (dm *DiskManager) Sync() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.closed {
		return ErrFileClosed
	}

	return dm.file.Sync()
}

func (dm *DiskManager) Close() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.closed {
		return nil
	}

	dm.closed = true

	if err := dm.file.Sync(); err != nil {
		return fmt.Errorf("file: failed to sync on close: %w", err)
	}

	return dm.file.Close()
}

func (dm *DiskManager) NumPages() uint32 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.numPages
}

func (dm *DiskManager) PageSize() int {
	return dm.pageSize
}

func (dm *DiskManager) FilePath() string {
	return dm.filePath
}

func (dm *DiskManager) FileSize() (int64, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.closed {
		return 0, ErrFileClosed
	}

	info, err := dm.file.Stat()
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}
