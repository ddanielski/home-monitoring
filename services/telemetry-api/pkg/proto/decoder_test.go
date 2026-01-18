package proto

import (
	"encoding/binary"
	"math"
	"testing"
)

// Helper to encode a varint
func encodeVarint(val uint64) []byte {
	var buf []byte
	for val >= 0x80 {
		buf = append(buf, byte(val)|0x80)
		val >>= 7
	}
	buf = append(buf, byte(val))
	return buf
}

// Helper to encode a tag
func encodeTag(fieldNum uint32, wireType WireType) []byte {
	return encodeVarint(uint64(fieldNum<<3) | uint64(wireType))
}

func TestDecode_Varint(t *testing.T) {
	// Field 1, varint, value = 150
	data := append(encodeTag(1, WireVarint), encodeVarint(150)...)

	msg, err := Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg) != 1 {
		t.Fatalf("expected 1 field, got %d", len(msg))
	}
	if msg[0].Num != 1 {
		t.Errorf("expected field 1, got %d", msg[0].Num)
	}
	if msg[0].Value.WireType != WireVarint {
		t.Errorf("expected varint wire type")
	}
	if msg[0].Value.Varint != 150 {
		t.Errorf("expected 150, got %d", msg[0].Value.Varint)
	}
}

func TestDecode_Fixed32(t *testing.T) {
	// Field 2, fixed32, value = 12345
	tag := encodeTag(2, WireFixed32)
	val := make([]byte, 4)
	binary.LittleEndian.PutUint32(val, 12345)
	data := append(tag, val...)

	msg, err := Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg) != 1 {
		t.Fatalf("expected 1 field, got %d", len(msg))
	}
	if msg[0].Value.Fixed32 != 12345 {
		t.Errorf("expected 12345, got %d", msg[0].Value.Fixed32)
	}
}

func TestDecode_Fixed64(t *testing.T) {
	// Field 3, fixed64
	tag := encodeTag(3, WireFixed64)
	val := make([]byte, 8)
	binary.LittleEndian.PutUint64(val, 9876543210)
	data := append(tag, val...)

	msg, err := Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg[0].Value.Fixed64 != 9876543210 {
		t.Errorf("expected 9876543210, got %d", msg[0].Value.Fixed64)
	}
}

func TestDecode_Bytes(t *testing.T) {
	// Field 4, bytes, value = "hello"
	tag := encodeTag(4, WireBytes)
	content := []byte("hello")
	length := encodeVarint(uint64(len(content)))
	data := append(tag, append(length, content...)...)

	msg, err := Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg[0].Value.Bytes) != "hello" {
		t.Errorf("expected 'hello', got %q", msg[0].Value.Bytes)
	}
}

func TestDecode_MultipleFields(t *testing.T) {
	var data []byte

	// Field 1, varint = 1
	data = append(data, encodeTag(1, WireVarint)...)
	data = append(data, encodeVarint(1)...)

	// Field 2, varint = 2
	data = append(data, encodeTag(2, WireVarint)...)
	data = append(data, encodeVarint(2)...)

	// Field 3, varint = 3
	data = append(data, encodeTag(3, WireVarint)...)
	data = append(data, encodeVarint(3)...)

	msg, err := Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(msg))
	}

	for i, f := range msg {
		if f.Num != uint32(i+1) {
			t.Errorf("field %d: expected num %d, got %d", i, i+1, f.Num)
		}
		if f.Value.Varint != uint64(i+1) {
			t.Errorf("field %d: expected value %d, got %d", i, i+1, f.Value.Varint)
		}
	}
}

func TestDecode_EmptyMessage(t *testing.T) {
	msg, err := Decode([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg) != 0 {
		t.Errorf("expected 0 fields, got %d", len(msg))
	}
}

func TestDecode_InvalidTag(t *testing.T) {
	// Invalid varint (all continuation bits set)
	data := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80}
	_, err := Decode(data)
	if err == nil {
		t.Error("expected error for invalid tag")
	}
}

func TestDecode_UnknownWireType(t *testing.T) {
	// Wire type 3 is deprecated "start group"
	data := encodeTag(1, 3)
	_, err := Decode(data)
	if err == nil {
		t.Error("expected error for unknown wire type")
	}
}

func TestDecode_TruncatedFixed32(t *testing.T) {
	tag := encodeTag(1, WireFixed32)
	data := append(tag, []byte{0x01, 0x02}...) // Only 2 bytes instead of 4

	_, err := Decode(data)
	if err == nil {
		t.Error("expected error for truncated fixed32")
	}
}

func TestDecode_TruncatedFixed64(t *testing.T) {
	tag := encodeTag(1, WireFixed64)
	data := append(tag, []byte{0x01, 0x02, 0x03, 0x04}...) // Only 4 bytes instead of 8

	_, err := Decode(data)
	if err == nil {
		t.Error("expected error for truncated fixed64")
	}
}

func TestDecode_TruncatedBytes(t *testing.T) {
	tag := encodeTag(1, WireBytes)
	length := encodeVarint(100) // Says 100 bytes
	data := append(tag, append(length, []byte("short")...)...)

	_, err := Decode(data)
	if err == nil {
		t.Error("expected error for truncated bytes")
	}
}

func TestMessage_GetField(t *testing.T) {
	msg := Message{
		{Num: 1, Value: Value{Varint: 10}},
		{Num: 2, Value: Value{Varint: 20}},
		{Num: 3, Value: Value{Varint: 30}},
	}

	f := msg.GetField(2)
	if f == nil {
		t.Fatal("expected to find field 2")
	}
	if f.Value.Varint != 20 {
		t.Errorf("expected 20, got %d", f.Value.Varint)
	}

	f = msg.GetField(99)
	if f != nil {
		t.Error("expected nil for non-existent field")
	}
}

func TestMessage_GetAllFields(t *testing.T) {
	msg := Message{
		{Num: 1, Value: Value{Varint: 10}},
		{Num: 2, Value: Value{Varint: 20}},
		{Num: 1, Value: Value{Varint: 11}}, // Repeated field 1
		{Num: 1, Value: Value{Varint: 12}}, // Repeated field 1
	}

	fields := msg.GetAllFields(1)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	fields = msg.GetAllFields(99)
	if len(fields) != 0 {
		t.Error("expected 0 fields for non-existent field")
	}
}

func TestDecodeRepeated(t *testing.T) {
	// Create an outer message with repeated embedded messages at field 1
	var outer []byte

	// Embedded message 1: field 1 = 100
	embedded1 := append(encodeTag(1, WireVarint), encodeVarint(100)...)
	outer = append(outer, encodeTag(1, WireBytes)...)
	outer = append(outer, encodeVarint(uint64(len(embedded1)))...)
	outer = append(outer, embedded1...)

	// Embedded message 2: field 1 = 200
	embedded2 := append(encodeTag(1, WireVarint), encodeVarint(200)...)
	outer = append(outer, encodeTag(1, WireBytes)...)
	outer = append(outer, encodeVarint(uint64(len(embedded2)))...)
	outer = append(outer, embedded2...)

	msgs, err := DecodeRepeated(outer, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].GetField(1).Value.Varint != 100 {
		t.Errorf("expected 100, got %d", msgs[0].GetField(1).Value.Varint)
	}
	if msgs[1].GetField(1).Value.Varint != 200 {
		t.Errorf("expected 200, got %d", msgs[1].GetField(1).Value.Varint)
	}
}

func TestValue_AsFloat32(t *testing.T) {
	val := Value{Fixed32: math.Float32bits(3.14)}
	result := val.AsFloat32()
	if result != 3.14 {
		t.Errorf("expected 3.14, got %f", result)
	}
}

func TestValue_AsFloat64(t *testing.T) {
	val := Value{Fixed64: math.Float64bits(3.14159)}
	result := val.AsFloat64()
	if result != 3.14159 {
		t.Errorf("expected 3.14159, got %f", result)
	}
}

func TestValue_AsInt32(t *testing.T) {
	// Store a positive value and test conversion
	val := Value{Varint: 100}
	result := val.AsInt32()
	if result != 100 {
		t.Errorf("expected 100, got %d", result)
	}
}

func TestValue_AsInt64(t *testing.T) {
	val := Value{Varint: 9876543210}
	result := val.AsInt64()
	if result != 9876543210 {
		t.Errorf("expected 9876543210, got %d", result)
	}
}

func TestValue_AsBool(t *testing.T) {
	val := Value{Varint: 1}
	if !val.AsBool() {
		t.Error("expected true")
	}

	val.Varint = 0
	if val.AsBool() {
		t.Error("expected false")
	}
}

func TestValue_AsSint32(t *testing.T) {
	// ZigZag encoding: -1 -> 1, -2 -> 3, 1 -> 2, 2 -> 4
	tests := []struct {
		zigzag   uint64
		expected int32
	}{
		{0, 0},
		{1, -1},
		{2, 1},
		{3, -2},
		{4, 2},
	}

	for _, tc := range tests {
		val := Value{Varint: tc.zigzag}
		result := val.AsSint32()
		if result != tc.expected {
			t.Errorf("AsSint32(%d) = %d, want %d", tc.zigzag, result, tc.expected)
		}
	}
}

func TestValue_AsString(t *testing.T) {
	val := Value{Bytes: []byte("hello world")}
	result := val.AsString()
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestValue_AsMessage(t *testing.T) {
	// Create embedded message: field 1 = 42
	embedded := append(encodeTag(1, WireVarint), encodeVarint(42)...)
	val := Value{Bytes: embedded}

	msg, err := val.AsMessage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.GetField(1).Value.Varint != 42 {
		t.Errorf("expected 42, got %d", msg.GetField(1).Value.Varint)
	}
}
