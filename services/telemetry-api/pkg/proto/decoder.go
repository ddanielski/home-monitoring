// Package proto provides a generic protobuf wire format decoder.
// It decodes raw protobuf bytes into structured data without requiring
// generated code or schema definitions at compile time.
package proto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// WireType represents protobuf wire types
type WireType uint8

const (
	WireVarint  WireType = 0
	WireFixed64 WireType = 1
	WireBytes   WireType = 2
	WireFixed32 WireType = 5
)

// Value represents a decoded protobuf field value with its wire type
type Value struct {
	WireType WireType
	Varint   uint64 // For WireVarint
	Fixed32  uint32 // For WireFixed32
	Fixed64  uint64 // For WireFixed64
	Bytes    []byte // For WireBytes (embedded messages, strings, bytes)
}

// Field represents a decoded protobuf field
type Field struct {
	Num   uint32
	Value Value
}

// Message represents a decoded protobuf message as a list of fields
type Message []Field

// Decode decodes raw protobuf wire format into a Message
func Decode(data []byte) (Message, error) {
	var fields Message
	pos := 0

	for pos < len(data) {
		tag, n := decodeVarint(data[pos:])
		if n == 0 {
			return nil, errors.New("failed to decode tag")
		}
		pos += n

		fieldNum := uint32(tag >> 3)
		wireType := WireType(tag & 0x7)

		field := Field{Num: fieldNum, Value: Value{WireType: wireType}}

		switch wireType {
		case WireVarint:
			val, n := decodeVarint(data[pos:])
			if n == 0 {
				return nil, fmt.Errorf("field %d: failed to decode varint", fieldNum)
			}
			field.Value.Varint = val
			pos += n

		case WireFixed64:
			if pos+8 > len(data) {
				return nil, fmt.Errorf("field %d: not enough data for fixed64", fieldNum)
			}
			field.Value.Fixed64 = binary.LittleEndian.Uint64(data[pos:])
			pos += 8

		case WireBytes:
			length, n := decodeVarint(data[pos:])
			if n == 0 {
				return nil, fmt.Errorf("field %d: failed to decode length", fieldNum)
			}
			pos += n
			if pos+int(length) > len(data) {
				return nil, fmt.Errorf("field %d: bytes length exceeds data", fieldNum)
			}
			field.Value.Bytes = data[pos : pos+int(length)]
			pos += int(length)

		case WireFixed32:
			if pos+4 > len(data) {
				return nil, fmt.Errorf("field %d: not enough data for fixed32", fieldNum)
			}
			field.Value.Fixed32 = binary.LittleEndian.Uint32(data[pos:])
			pos += 4

		default:
			return nil, fmt.Errorf("field %d: unknown wire type %d", fieldNum, wireType)
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// GetField returns the first field with the given field number, or nil if not found
func (m Message) GetField(fieldNum uint32) *Field {
	for i := range m {
		if m[i].Num == fieldNum {
			return &m[i]
		}
	}
	return nil
}

// GetAllFields returns all fields with the given field number (for repeated fields)
func (m Message) GetAllFields(fieldNum uint32) []Field {
	var fields []Field
	for _, f := range m {
		if f.Num == fieldNum {
			fields = append(fields, f)
		}
	}
	return fields
}

// DecodeRepeated decodes a message containing repeated embedded messages at the given field number.
// Returns a slice of Message, one for each embedded message.
func DecodeRepeated(data []byte, fieldNum uint32) ([]Message, error) {
	msg, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode outer message: %w", err)
	}

	fields := msg.GetAllFields(fieldNum)
	result := make([]Message, 0, len(fields))

	for _, field := range fields {
		if field.Value.WireType != WireBytes {
			continue
		}
		embedded, err := Decode(field.Value.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to decode embedded message: %w", err)
		}
		result = append(result, embedded)
	}

	return result, nil
}

// Value interpretation helpers - the caller decides the semantic type

// AsFloat32 interprets a Fixed32 value as float32
func (v Value) AsFloat32() float32 {
	return math.Float32frombits(v.Fixed32)
}

// AsFloat64 interprets a Fixed64 value as float64
func (v Value) AsFloat64() float64 {
	return math.Float64frombits(v.Fixed64)
}

// AsInt32 interprets a Varint value as int32
func (v Value) AsInt32() int32 {
	return int32(v.Varint)
}

// AsInt64 interprets a Varint value as int64
func (v Value) AsInt64() int64 {
	return int64(v.Varint)
}

// AsUint32 interprets a Varint value as uint32
func (v Value) AsUint32() uint32 {
	return uint32(v.Varint)
}

// AsUint64 returns the Varint value as uint64
func (v Value) AsUint64() uint64 {
	return v.Varint
}

// AsBool interprets a Varint value as bool
func (v Value) AsBool() bool {
	return v.Varint != 0
}

// AsSint32 interprets a Varint value as sint32 (ZigZag decoded)
func (v Value) AsSint32() int32 {
	return int32((v.Varint >> 1) ^ -(v.Varint & 1))
}

// AsSint64 interprets a Varint value as sint64 (ZigZag decoded)
func (v Value) AsSint64() int64 {
	return int64((v.Varint >> 1) ^ -(v.Varint & 1))
}

// AsString interprets Bytes as a UTF-8 string
func (v Value) AsString() string {
	return string(v.Bytes)
}

// AsMessage decodes Bytes as an embedded protobuf message
func (v Value) AsMessage() (Message, error) {
	return Decode(v.Bytes)
}

func decodeVarint(data []byte) (uint64, int) {
	var result uint64
	var shift uint
	for i, b := range data {
		if i >= 10 {
			return 0, 0 // overflow
		}
		result |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return result, i + 1
		}
		shift += 7
	}
	return 0, 0
}
