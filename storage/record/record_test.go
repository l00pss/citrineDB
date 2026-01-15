package record

import (
	"math"
	"testing"
)

func TestRecordID(t *testing.T) {
	rid := NewRecordID(1, 5)
	if rid.PageID != 1 || rid.SlotID != 5 {
		t.Errorf("expected (1, 5), got (%d, %d)", rid.PageID, rid.SlotID)
	}
	if !rid.IsValid() {
		t.Error("expected valid record ID")
	}
	zeroRID := RecordID{}
	if zeroRID.IsValid() {
		t.Error("expected invalid record ID for zero value")
	}
}

func TestSchema(t *testing.T) {
	fields := []Field{
		{Name: "id", Type: FieldTypeInt64, Nullable: false},
		{Name: "name", Type: FieldTypeString, Nullable: false, MaxLen: 100},
		{Name: "age", Type: FieldTypeInt32, Nullable: true},
	}
	schema := NewSchema(fields)
	if schema.FieldCount() != 3 {
		t.Errorf("expected 3 fields, got %d", schema.FieldCount())
	}
	idx, ok := schema.FieldIndex("name")
	if !ok || idx != 1 {
		t.Errorf("expected index 1 for 'name', got %d", idx)
	}
	_, ok = schema.FieldIndex("unknown")
	if ok {
		t.Error("expected false for unknown field")
	}
	field, err := schema.Field(0)
	if err != nil || field.Name != "id" {
		t.Error("failed to get field by index")
	}
	_, err = schema.Field(-1)
	if err != ErrInvalidFieldIdx {
		t.Error("expected ErrInvalidFieldIdx for negative index")
	}
}

func TestFieldType(t *testing.T) {
	tests := []struct {
		ft     FieldType
		size   int
		strVal string
	}{
		{FieldTypeNull, 0, "NULL"},
		{FieldTypeInt8, 1, "INT8"},
		{FieldTypeInt16, 2, "INT16"},
		{FieldTypeInt32, 4, "INT32"},
		{FieldTypeInt64, 8, "INT64"},
		{FieldTypeFloat32, 4, "FLOAT32"},
		{FieldTypeFloat64, 8, "FLOAT64"},
		{FieldTypeBool, 1, "BOOL"},
		{FieldTypeString, -1, "STRING"},
		{FieldTypeBytes, -1, "BYTES"},
	}
	for _, tt := range tests {
		if got := tt.ft.FixedSize(); got != tt.size {
			t.Errorf("%s: expected size %d, got %d", tt.ft, tt.size, got)
		}
		if got := tt.ft.String(); got != tt.strVal {
			t.Errorf("expected string %s, got %s", tt.strVal, got)
		}
	}
}

func TestIntValues(t *testing.T) {
	v8 := Int8Value(42)
	got8, _ := v8.AsInt8()
	if got8 != 42 {
		t.Errorf("Int8: expected 42, got %d", got8)
	}
	v16 := Int16Value(1000)
	got16, _ := v16.AsInt16()
	if got16 != 1000 {
		t.Errorf("Int16: expected 1000, got %d", got16)
	}
	v32 := Int32Value(100000)
	got32, _ := v32.AsInt32()
	if got32 != 100000 {
		t.Errorf("Int32: expected 100000, got %d", got32)
	}
	v64 := Int64Value(9999999999)
	got64, _ := v64.AsInt64()
	if got64 != 9999999999 {
		t.Errorf("Int64: expected 9999999999, got %d", got64)
	}
}

func TestFloatValues(t *testing.T) {
	v32 := Float32Value(3.14)
	got32, _ := v32.AsFloat32()
	if math.Abs(float64(got32-3.14)) > 0.001 {
		t.Errorf("Float32: expected ~3.14, got %f", got32)
	}
	v64 := Float64Value(3.14159265359)
	got64, _ := v64.AsFloat64()
	if math.Abs(got64-3.14159265359) > 0.0000001 {
		t.Errorf("Float64: expected ~3.14159, got %f", got64)
	}
}

func TestOtherValues(t *testing.T) {
	vBool := BoolValue(true)
	gotBool, _ := vBool.AsBool()
	if !gotBool {
		t.Error("Bool: expected true")
	}
	vStr := StringValue("hello")
	gotStr, _ := vStr.AsString()
	if gotStr != "hello" {
		t.Errorf("String: expected 'hello', got '%s'", gotStr)
	}
	vBytes := BytesValue([]byte{1, 2, 3})
	gotBytes, _ := vBytes.AsBytes()
	if len(gotBytes) != 3 {
		t.Error("Bytes: mismatch")
	}
}

func TestNullValue(t *testing.T) {
	v := NullValue()
	if !v.IsNull {
		t.Error("expected null value")
	}
	_, err := v.AsInt64()
	if err != ErrNullValue {
		t.Error("expected ErrNullValue")
	}
}

func TestTypeMismatch(t *testing.T) {
	v := Int32Value(42)
	_, err := v.AsString()
	if err != ErrTypeMismatch {
		t.Error("expected ErrTypeMismatch")
	}
}

func TestRecordSetGet(t *testing.T) {
	schema := NewSchema([]Field{
		{Name: "id", Type: FieldTypeInt64},
		{Name: "name", Type: FieldTypeString},
	})
	r := NewRecord(schema)
	r.Set(0, Int64Value(1))
	r.SetByName("name", StringValue("Alice"))
	v1, _ := r.Get(0)
	id, _ := v1.AsInt64()
	if id != 1 {
		t.Errorf("expected id=1, got %d", id)
	}
	v2, _ := r.GetByName("name")
	name, _ := v2.AsString()
	if name != "Alice" {
		t.Errorf("expected name='Alice', got '%s'", name)
	}
	_, err := r.GetByName("unknown")
	if err != ErrFieldNotFound {
		t.Error("expected ErrFieldNotFound")
	}
	err = r.Set(-1, Int64Value(0))
	if err != ErrInvalidFieldIdx {
		t.Error("expected ErrInvalidFieldIdx")
	}
}

func TestRecordSerializeDeserialize(t *testing.T) {
	schema := NewSchema([]Field{
		{Name: "id", Type: FieldTypeInt64},
		{Name: "name", Type: FieldTypeString},
		{Name: "age", Type: FieldTypeInt32, Nullable: true},
		{Name: "active", Type: FieldTypeBool},
	})
	r := NewRecord(schema)
	r.Set(0, Int64Value(42))
	r.Set(1, StringValue("Bob"))
	r.Set(2, NullValue())
	r.Set(3, BoolValue(true))
	data := r.Serialize()
	r2, err := DeserializeRecord(schema, data)
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}
	v, _ := r2.Get(0)
	id, _ := v.AsInt64()
	if id != 42 {
		t.Errorf("expected id=42, got %d", id)
	}
	v, _ = r2.Get(1)
	name, _ := v.AsString()
	if name != "Bob" {
		t.Errorf("expected name='Bob', got '%s'", name)
	}
	v, _ = r2.Get(2)
	if !v.IsNull {
		t.Error("expected age to be null")
	}
	v, _ = r2.Get(3)
	active, _ := v.AsBool()
	if !active {
		t.Error("expected active=true")
	}
}

func TestDeserializeErrors(t *testing.T) {
	schema := NewSchema([]Field{{Name: "id", Type: FieldTypeInt64}})
	_, err := DeserializeRecord(schema, []byte{})
	if err != ErrInvalidRecord {
		t.Error("expected ErrInvalidRecord for empty data")
	}
	_, err = DeserializeRecord(schema, []byte{0x00})
	if err != ErrInvalidRecord {
		t.Error("expected ErrInvalidRecord for truncated data")
	}
}

func BenchmarkRecordSerialize(b *testing.B) {
	schema := NewSchema([]Field{
		{Name: "id", Type: FieldTypeInt64},
		{Name: "name", Type: FieldTypeString},
		{Name: "age", Type: FieldTypeInt32},
		{Name: "active", Type: FieldTypeBool},
	})
	r := NewRecord(schema)
	r.Set(0, Int64Value(12345))
	r.Set(1, StringValue("John Doe"))
	r.Set(2, Int32Value(30))
	r.Set(3, BoolValue(true))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Serialize()
	}
}

func BenchmarkRecordDeserialize(b *testing.B) {
	schema := NewSchema([]Field{
		{Name: "id", Type: FieldTypeInt64},
		{Name: "name", Type: FieldTypeString},
		{Name: "age", Type: FieldTypeInt32},
		{Name: "active", Type: FieldTypeBool},
	})
	r := NewRecord(schema)
	r.Set(0, Int64Value(12345))
	r.Set(1, StringValue("John Doe"))
	r.Set(2, Int32Value(30))
	r.Set(3, BoolValue(true))
	data := r.Serialize()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DeserializeRecord(schema, data)
	}
}
