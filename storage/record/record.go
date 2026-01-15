package record

import (
	"encoding/binary"
	"errors"
	"math"
)

var (
	ErrInvalidRecord   = errors.New("record: invalid record data")
	ErrFieldNotFound   = errors.New("record: field not found")
	ErrNullValue       = errors.New("record: null value")
	ErrTypeMismatch    = errors.New("record: type mismatch")
	ErrInvalidFieldIdx = errors.New("record: invalid field index")
)

type RecordID struct {
	PageID uint32
	SlotID uint16
}

func NewRecordID(pageID uint32, slotID uint16) RecordID {
	return RecordID{PageID: pageID, SlotID: slotID}
}

func (rid RecordID) IsValid() bool {
	return rid.PageID != 0 || rid.SlotID != 0
}

type FieldType uint8

const (
	FieldTypeNull FieldType = iota
	FieldTypeInt8
	FieldTypeInt16
	FieldTypeInt32
	FieldTypeInt64
	FieldTypeFloat32
	FieldTypeFloat64
	FieldTypeBool
	FieldTypeString
	FieldTypeBytes
)

func (ft FieldType) String() string {
	names := []string{"NULL", "INT8", "INT16", "INT32", "INT64", "FLOAT32", "FLOAT64", "BOOL", "STRING", "BYTES"}
	if int(ft) < len(names) {
		return names[ft]
	}
	return "UNKNOWN"
}

func (ft FieldType) FixedSize() int {
	switch ft {
	case FieldTypeNull:
		return 0
	case FieldTypeInt8, FieldTypeBool:
		return 1
	case FieldTypeInt16:
		return 2
	case FieldTypeInt32, FieldTypeFloat32:
		return 4
	case FieldTypeInt64, FieldTypeFloat64:
		return 8
	default:
		return -1
	}
}

type Field struct {
	Name     string
	Type     FieldType
	Nullable bool
	MaxLen   int
}

type Schema struct {
	Fields     []Field
	fieldIndex map[string]int
}

func NewSchema(fields []Field) *Schema {
	s := &Schema{
		Fields:     fields,
		fieldIndex: make(map[string]int),
	}
	for i, f := range fields {
		s.fieldIndex[f.Name] = i
	}
	return s
}

func (s *Schema) FieldCount() int {
	return len(s.Fields)
}

func (s *Schema) FieldIndex(name string) (int, bool) {
	idx, ok := s.fieldIndex[name]
	return idx, ok
}

func (s *Schema) Field(idx int) (*Field, error) {
	if idx < 0 || idx >= len(s.Fields) {
		return nil, ErrInvalidFieldIdx
	}
	return &s.Fields[idx], nil
}

type Value struct {
	Type     FieldType
	IsNull   bool
	Data     []byte
	intVal   int64
	floatVal float64
	boolVal  bool
}

func NullValue() Value             { return Value{Type: FieldTypeNull, IsNull: true} }
func Int8Value(v int8) Value       { return Value{Type: FieldTypeInt8, intVal: int64(v)} }
func Int16Value(v int16) Value     { return Value{Type: FieldTypeInt16, intVal: int64(v)} }
func Int32Value(v int32) Value     { return Value{Type: FieldTypeInt32, intVal: int64(v)} }
func Int64Value(v int64) Value     { return Value{Type: FieldTypeInt64, intVal: v} }
func Float32Value(v float32) Value { return Value{Type: FieldTypeFloat32, floatVal: float64(v)} }
func Float64Value(v float64) Value { return Value{Type: FieldTypeFloat64, floatVal: v} }
func BoolValue(v bool) Value       { return Value{Type: FieldTypeBool, boolVal: v} }
func StringValue(v string) Value   { return Value{Type: FieldTypeString, Data: []byte(v)} }
func BytesValue(v []byte) Value {
	data := make([]byte, len(v))
	copy(data, v)
	return Value{Type: FieldTypeBytes, Data: data}
}

func (v Value) AsInt8() (int8, error) {
	if v.IsNull {
		return 0, ErrNullValue
	}
	if v.Type != FieldTypeInt8 {
		return 0, ErrTypeMismatch
	}
	return int8(v.intVal), nil
}

func (v Value) AsInt16() (int16, error) {
	if v.IsNull {
		return 0, ErrNullValue
	}
	if v.Type != FieldTypeInt16 {
		return 0, ErrTypeMismatch
	}
	return int16(v.intVal), nil
}

func (v Value) AsInt32() (int32, error) {
	if v.IsNull {
		return 0, ErrNullValue
	}
	if v.Type != FieldTypeInt32 {
		return 0, ErrTypeMismatch
	}
	return int32(v.intVal), nil
}

func (v Value) AsInt64() (int64, error) {
	if v.IsNull {
		return 0, ErrNullValue
	}
	if v.Type != FieldTypeInt64 {
		return 0, ErrTypeMismatch
	}
	return v.intVal, nil
}

func (v Value) AsFloat32() (float32, error) {
	if v.IsNull {
		return 0, ErrNullValue
	}
	if v.Type != FieldTypeFloat32 {
		return 0, ErrTypeMismatch
	}
	return float32(v.floatVal), nil
}

func (v Value) AsFloat64() (float64, error) {
	if v.IsNull {
		return 0, ErrNullValue
	}
	if v.Type != FieldTypeFloat64 {
		return 0, ErrTypeMismatch
	}
	return v.floatVal, nil
}

func (v Value) AsBool() (bool, error) {
	if v.IsNull {
		return false, ErrNullValue
	}
	if v.Type != FieldTypeBool {
		return false, ErrTypeMismatch
	}
	return v.boolVal, nil
}

func (v Value) AsString() (string, error) {
	if v.IsNull {
		return "", ErrNullValue
	}
	if v.Type != FieldTypeString {
		return "", ErrTypeMismatch
	}
	return string(v.Data), nil
}

func (v Value) AsBytes() ([]byte, error) {
	if v.IsNull {
		return nil, ErrNullValue
	}
	if v.Type != FieldTypeBytes {
		return nil, ErrTypeMismatch
	}
	return v.Data, nil
}

type Record struct {
	schema *Schema
	values []Value
}

func NewRecord(schema *Schema) *Record {
	return &Record{schema: schema, values: make([]Value, schema.FieldCount())}
}

func (r *Record) Schema() *Schema { return r.schema }

func (r *Record) Set(idx int, value Value) error {
	if idx < 0 || idx >= len(r.values) {
		return ErrInvalidFieldIdx
	}
	r.values[idx] = value
	return nil
}

func (r *Record) SetByName(name string, value Value) error {
	idx, ok := r.schema.FieldIndex(name)
	if !ok {
		return ErrFieldNotFound
	}
	return r.Set(idx, value)
}

func (r *Record) Get(idx int) (Value, error) {
	if idx < 0 || idx >= len(r.values) {
		return Value{}, ErrInvalidFieldIdx
	}
	return r.values[idx], nil
}

func (r *Record) GetByName(name string) (Value, error) {
	idx, ok := r.schema.FieldIndex(name)
	if !ok {
		return Value{}, ErrFieldNotFound
	}
	return r.Get(idx)
}

func (r *Record) Serialize() []byte {
	buf := make([]byte, 4096)
	offset := 0
	nullBitmapSize := (len(r.values) + 7) / 8
	nullBitmap := buf[offset : offset+nullBitmapSize]
	offset += nullBitmapSize

	for i, v := range r.values {
		if v.IsNull {
			byteIdx := i / 8
			bitIdx := uint(i % 8)
			nullBitmap[byteIdx] |= (1 << bitIdx)
		}
	}

	for i, v := range r.values {
		if v.IsNull {
			continue
		}
		field := r.schema.Fields[i]
		switch field.Type {
		case FieldTypeInt8:
			buf[offset] = byte(v.intVal)
			offset++
		case FieldTypeInt16:
			binary.LittleEndian.PutUint16(buf[offset:], uint16(v.intVal))
			offset += 2
		case FieldTypeInt32:
			binary.LittleEndian.PutUint32(buf[offset:], uint32(v.intVal))
			offset += 4
		case FieldTypeInt64:
			binary.LittleEndian.PutUint64(buf[offset:], uint64(v.intVal))
			offset += 8
		case FieldTypeFloat32:
			binary.LittleEndian.PutUint32(buf[offset:], math.Float32bits(float32(v.floatVal)))
			offset += 4
		case FieldTypeFloat64:
			binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(v.floatVal))
			offset += 8
		case FieldTypeBool:
			if v.boolVal {
				buf[offset] = 1
			} else {
				buf[offset] = 0
			}
			offset++
		case FieldTypeString, FieldTypeBytes:
			binary.LittleEndian.PutUint16(buf[offset:], uint16(len(v.Data)))
			offset += 2
			copy(buf[offset:], v.Data)
			offset += len(v.Data)
		}
	}
	return buf[:offset]
}

func DeserializeRecord(schema *Schema, data []byte) (*Record, error) {
	if len(data) == 0 {
		return nil, ErrInvalidRecord
	}
	r := NewRecord(schema)
	offset := 0
	nullBitmapSize := (schema.FieldCount() + 7) / 8
	if len(data) < nullBitmapSize {
		return nil, ErrInvalidRecord
	}
	nullBitmap := data[offset : offset+nullBitmapSize]
	offset += nullBitmapSize

	for i := 0; i < schema.FieldCount(); i++ {
		byteIdx := i / 8
		bitIdx := uint(i % 8)
		isNull := (nullBitmap[byteIdx] & (1 << bitIdx)) != 0
		if isNull {
			r.values[i] = NullValue()
			continue
		}
		field := schema.Fields[i]
		switch field.Type {
		case FieldTypeInt8:
			if offset >= len(data) {
				return nil, ErrInvalidRecord
			}
			r.values[i] = Int8Value(int8(data[offset]))
			offset++
		case FieldTypeInt16:
			if offset+2 > len(data) {
				return nil, ErrInvalidRecord
			}
			r.values[i] = Int16Value(int16(binary.LittleEndian.Uint16(data[offset:])))
			offset += 2
		case FieldTypeInt32:
			if offset+4 > len(data) {
				return nil, ErrInvalidRecord
			}
			r.values[i] = Int32Value(int32(binary.LittleEndian.Uint32(data[offset:])))
			offset += 4
		case FieldTypeInt64:
			if offset+8 > len(data) {
				return nil, ErrInvalidRecord
			}
			r.values[i] = Int64Value(int64(binary.LittleEndian.Uint64(data[offset:])))
			offset += 8
		case FieldTypeFloat32:
			if offset+4 > len(data) {
				return nil, ErrInvalidRecord
			}
			bits := binary.LittleEndian.Uint32(data[offset:])
			r.values[i] = Float32Value(math.Float32frombits(bits))
			offset += 4
		case FieldTypeFloat64:
			if offset+8 > len(data) {
				return nil, ErrInvalidRecord
			}
			bits := binary.LittleEndian.Uint64(data[offset:])
			r.values[i] = Float64Value(math.Float64frombits(bits))
			offset += 8
		case FieldTypeBool:
			if offset >= len(data) {
				return nil, ErrInvalidRecord
			}
			r.values[i] = BoolValue(data[offset] != 0)
			offset++
		case FieldTypeString:
			if offset+2 > len(data) {
				return nil, ErrInvalidRecord
			}
			length := int(binary.LittleEndian.Uint16(data[offset:]))
			offset += 2
			if offset+length > len(data) {
				return nil, ErrInvalidRecord
			}
			r.values[i] = StringValue(string(data[offset : offset+length]))
			offset += length
		case FieldTypeBytes:
			if offset+2 > len(data) {
				return nil, ErrInvalidRecord
			}
			length := int(binary.LittleEndian.Uint16(data[offset:]))
			offset += 2
			if offset+length > len(data) {
				return nil, ErrInvalidRecord
			}
			r.values[i] = BytesValue(data[offset : offset+length])
			offset += length
		}
	}
	return r, nil
}
