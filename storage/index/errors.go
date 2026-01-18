package index

import "errors"

var (
	ErrDuplicateKey  = errors.New("index: duplicate key violation")
	ErrKeyNotFound   = errors.New("index: key not found")
	ErrIndexNotFound = errors.New("index: index not found")
)
