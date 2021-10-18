package mapper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

func TestMapReaderDataTypes(t *testing.T) {
	templateData := `
structs:
  header:
    u8 U8 single: fb
    u8[2] U8 array: ff d8
    u8[0] Empty data: ??

    endian: big
    u16 U16BE single: aaf0
    u16[2] U16BE array: 1011 2021
    u32 U32BE single: "11223344"
    u32[2] U32BE array: 10111213 20212223
    u64 U64BE single: "1122334455667788"
    u64[2] U64BE array: 1011121314151617 2021222324252627

    endian: little
    u16 U16LE single: aaf0
    u16[2] U16LE array: 1011 2021
    u32 U32LE single: "11223344"
    u32[2] U32LE array: 10111213 20212223
    u64 U64LE single: "1122334455667788"
    u64[2] U64LE array: 1011121314151617 2021222324252627

    time_t_32 TimeT_32_LE: 525e65ef

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		// bytes
		0xfb,       // u8 single
		0xff, 0xd8, // u8[2] array

		// big endian
		0xaa, 0xf0, // U16BE single
		0x10, 0x11, 0x20, 0x21, // u16[2] U16BE array
		0x11, 0x22, 0x33, 0x44, // U32BE single
		0x10, 0x11, 0x12, 0x13, 0x20, 0x21, 0x22, 0x23, // u32[2] U32BE array
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, // U64BE single
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, // u64[2] U64BE array

		// little endian
		0xf0, 0xaa, // U16LE single
		0x11, 0x10, 0x21, 0x20, // u16[2] U16LE array
		0x44, 0x33, 0x22, 0x11, // U32LE single
		0x13, 0x12, 0x11, 0x10, 0x23, 0x22, 0x21, 0x20, // u32[2] U32LE array
		0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, // U64LE single
		0x17, 0x16, 0x15, 0x14, 0x13, 0x12, 0x11, 0x10, 0x27, 0x26, 0x25, 0x24, 0x23, 0x22, 0x21, 0x20, // u64[2] U64LE array

		0xef, 0x65, 0x5e, 0x52, // TimeT_32_LE
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{Label: "Header", Fields: []Field{
					{Offset: 0x0, Length: 0x1, Value: []uint8{0xfb}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "U8 single"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x1, Length: 0x2, Value: []uint8{0xff, 0xd8}, Endian: "", Format: value.DataField{Kind: "u8", Range: "2", Slice: false, Label: "U8 array"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x3, Length: 0x2, Value: []uint8{0xaa, 0xf0}, Endian: "big", Format: value.DataField{Kind: "u16", Range: "", Slice: false, Label: "U16BE single"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x5, Length: 0x4, Value: []uint8{0x10, 0x11, 0x20, 0x21}, Endian: "big", Format: value.DataField{Kind: "u16", Range: "2", Slice: false, Label: "U16BE array"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x9, Length: 0x4, Value: []uint8{0x11, 0x22, 0x33, 0x44}, Endian: "big", Format: value.DataField{Kind: "u32", Range: "", Slice: false, Label: "U32BE single"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0xd, Length: 0x8, Value: []uint8{0x10, 0x11, 0x12, 0x13, 0x20, 0x21, 0x22, 0x23}, Endian: "big", Format: value.DataField{Kind: "u32", Range: "2", Slice: false, Label: "U32BE array"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x15, Length: 0x8, Value: []uint8{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}, Endian: "big", Format: value.DataField{Kind: "u64", Range: "", Slice: false, Label: "U64BE single"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x1d, Length: 0x10, Value: []uint8{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27}, Endian: "big", Format: value.DataField{Kind: "u64", Range: "2", Slice: false, Label: "U64BE array"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x2d, Length: 0x2, Value: []uint8{0xaa, 0xf0}, Endian: "little", Format: value.DataField{Kind: "u16", Range: "", Slice: false, Label: "U16LE single"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x2f, Length: 0x4, Value: []uint8{0x10, 0x11, 0x20, 0x21}, Endian: "little", Format: value.DataField{Kind: "u16", Range: "2", Slice: false, Label: "U16LE array"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x33, Length: 0x4, Value: []uint8{0x11, 0x22, 0x33, 0x44}, Endian: "little", Format: value.DataField{Kind: "u32", Range: "", Slice: false, Label: "U32LE single"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x37, Length: 0x8, Value: []uint8{0x10, 0x11, 0x12, 0x13, 0x20, 0x21, 0x22, 0x23}, Endian: "little", Format: value.DataField{Kind: "u32", Range: "2", Slice: false, Label: "U32LE array"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x3f, Length: 0x8, Value: []uint8{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}, Endian: "little", Format: value.DataField{Kind: "u64", Range: "", Slice: false, Label: "U64LE single"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x47, Length: 0x10, Value: []uint8{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27}, Endian: "little", Format: value.DataField{Kind: "u64", Range: "2", Slice: false, Label: "U64LE array"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x57, Length: 0x04, Value: []uint8{0x52, 0x5e, 0x65, 0xef}, Endian: "little", Format: value.DataField{Kind: "time_t_32", Range: "", Slice: false, Label: "TimeT_32_LE"}, MatchedPatterns: []value.MatchedPattern{}},
				}}},
			endian: "little", offset: 0x5b, size: 0x5b}, fl)
}

func TestMapReaderMatchPatterns(t *testing.T) {
	templateData := `
structs:
  header:
    u8 Sig8a: fb
    u8[2] Sig8b: ff d8

    endian: big
    u16 Sig16be: ??
    u16[2] Sig16be_array: ??
    u32 U32BE single: ??
    u32[2] U32BE array: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	test := []struct {
		in  []byte
		out *FileLayout
		err error
	}{
		{[]byte{}, nil, io.EOF},

		{[]byte{
			// expected bytes
			0xfb, 0xff, 0xd8, // u8 single, u8[2] array
			0x4a, 0xf0, // u16 single
			0x16, 0x17, 0x16, 0x17, // u16[2] array
			0x12, 0x34, 0x56, 0x78, // u32 single
			0x22, 0x22, 0x33, 0x33, 0x44, 0x44, 0x55, 0x55, // u32[2] array
		},
			&FileLayout{
				Structs: []Struct{
					{Label: "Header", Fields: []Field{
						{Offset: 0x0, Length: 0x1, Value: []uint8{0xfb}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Sig8a"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x1, Length: 0x2, Value: []uint8{0xff, 0xd8}, Endian: "", Format: value.DataField{Kind: "u8", Range: "2", Slice: false, Label: "Sig8b"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x3, Length: 0x2, Value: []uint8{0x4a, 0xf0}, Endian: "big", Format: value.DataField{Kind: "u16", Range: "", Slice: false, Label: "Sig16be"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x5, Length: 0x4, Value: []uint8{0x16, 0x17, 0x16, 0x17}, Endian: "big", Format: value.DataField{Kind: "u16", Range: "2", Slice: false, Label: "Sig16be_array"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x9, Length: 0x4, Value: []uint8{0x12, 0x34, 0x56, 0x78}, Endian: "big", Format: value.DataField{Kind: "u32", Range: "", Slice: false, Label: "U32BE single"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0xd, Length: 0x8, Value: []uint8{0x22, 0x22, 0x33, 0x33, 0x44, 0x44, 0x55, 0x55}, Endian: "big", Format: value.DataField{Kind: "u32", Range: "2", Slice: false, Label: "U32BE array"}, MatchedPatterns: []value.MatchedPattern{}},
					}}},
				offset: 0x15, size: 0x15, endian: "big"}, nil},

		{[]byte{
			// wrong first byte
			0xfa, 0xff, 0xd8, // u8 single,u8[2] array
		}, nil, errors.New("[00000000] pattern 'Sig8a' does not match. expected 'fb', got 'fa'")},

		{[]byte{
			// wrong second byte
			0xfb, 0xfe, 0xd8, // u8 single, u8[2] array
		}, nil, errors.New("[00000001] pattern 'Sig8b' does not match. expected 'ff d8', got 'fe d8'")},

		{[]byte{
			// wrong third byte
			0xfb, 0xff, 0xdd, // u8 single, u8[2] array
		}, nil, errors.New("[00000001] pattern 'Sig8b' does not match. expected 'ff d8', got 'ff dd'")},
	}

	for _, tt := range test {
		fl, err := MapReader(bytes.NewReader(tt.in), ds)
		assert.Equal(t, tt.err, err)
		if tt.err == nil {
			assert.Equal(t, tt.out, fl)
		}
	}
}

func TestEvaluateBitFieldU8(t *testing.T) {
	templateData := `
structs:
  header:
    u8 Bitfield:
      bit b0000_0001: LocalColorTable
      bit b0000_0010: Interlace
      bit b0000_0100: Sort
      bit b0001_1000: Reserved
      bit b1110_0000: Size
layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		0b1100_0111, // Bitfield
	}
	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{Label: "Header", Fields: []Field{
					{
						Offset: 0x0, Length: 0x1, Value: []uint8{0xc7}, Endian: "",
						Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Bitfield"},
						MatchedPatterns: []value.MatchedPattern{
							{Operation: "bit", Label: "LocalColorTable", Value: 1},
							{Operation: "bit", Label: "Interlace", Value: 1},
							{Operation: "bit", Label: "Sort", Value: 1},
							{Operation: "bit", Label: "Reserved", Value: 0},
							{Operation: "bit", Label: "Size", Value: 6},
						}},
				}}}, offset: 0x1, size: 1}, fl)
}

func TestEvaluateBitFieldU16(t *testing.T) {
	templateData := `
structs:
  header:
    endian: little
    u16 Bitfield:
      bit b0000_0000_0111_1111: Lo
      bit b0000_1111_1000_0000: B3
      bit b1111_0000_0000_0000: Hi
layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		0xff, 0xff, // Bitfield
	}
	ff, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{Label: "Header", Fields: []Field{
					{
						Offset: 0x0, Length: 0x2, Value: []uint8{0xff, 0xff}, Endian: "little",
						Format: value.DataField{Kind: "u16", Range: "", Slice: false, Label: "Bitfield"},
						MatchedPatterns: []value.MatchedPattern{
							{Operation: "bit", Label: "Lo", Value: 0x7f},
							{Operation: "bit", Label: "B3", Value: 0x1f},
							{Operation: "bit", Label: "Hi", Value: 0xf},
						}},
				}}},
			offset: 0x2, size: 0x2, endian: "little"}, ff)
}

func TestEvaluateEqFieldU8(t *testing.T) {
	templateData := `
structs:
  header:
    u8 Field:
      eq 00: No units
      eq 01: One
      eq c'a': Letter A
      default: invalid
layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	test := []struct {
		in  []byte
		out *FileLayout
		err error
	}{
		{
			[]byte{
				0x01, // field
			},
			&FileLayout{
				Structs: []Struct{
					{Label: "Header", Fields: []Field{
						{Offset: 0x0, Length: 0x1, Value: []uint8{0x01}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Field"},
							MatchedPatterns: []value.MatchedPattern{
								{Operation: "eq", Label: "One", Value: 1},
							}},
					}}}, offset: 0x1, size: 1},
			nil,
		},
		{
			[]byte{
				0x03, // invalid field
			},
			nil,
			fmt.Errorf("value 00000003 (3) for Field is not valid"),
		},
	}

	for _, tt := range test {
		fl, err := MapReader(bytes.NewReader(tt.in), ds)
		assert.Equal(t, tt.err, err)
		if tt.err == nil {
			assert.Equal(t, tt.out, fl)
		}
	}
}

func TestExpandBitfieldValue(t *testing.T) {
	templateData := `
structs:
  header:
    u8 Field:
      bit b0000_0011: Size
    u8[1 << self.Field.Size] Data: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		0xff,                                           // Field
		0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, // Data
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	s, err := fl.ExpandVariables("Header.Field.Size", &fl.Structs[0].Fields[0].Format)
	assert.Equal(t, nil, err)
	assert.Equal(t, "3", s)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{Label: "Header",
					Fields: []Field{
						{Offset: 0x0, Length: 0x1, Value: []uint8{0xff}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Field"}, MatchedPatterns: []value.MatchedPattern{{Label: "Size", Operation: "bit", Value: 0x3}}},
						{Offset: 0x1, Length: 0x8, Value: []uint8{0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7}, Endian: "", Format: value.DataField{Kind: "u8", Range: "1 <<3", Slice: false, Label: "Data"}, MatchedPatterns: []value.MatchedPattern{}},
					},
				}}, offset: 0x9, size: 9}, fl)
}

func TestExpandStructValue(t *testing.T) {
	templateData := `
structs:
  header:
    endian: little
    u16 Length: ??
    u8[self.Length] Data: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		0x02, 0x00, // Length
		0x44, 0x55, // Data
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{Label: "Header",
					Fields: []Field{
						{Offset: 0x0, Length: 0x2, Value: []uint8{0x0, 0x2}, Endian: "little", Format: value.DataField{Kind: "u16", Range: "", Slice: false, Label: "Length"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x2, Length: 0x2, Value: []uint8{0x44, 0x55}, Endian: "little", Format: value.DataField{Kind: "u8", Range: "2", Slice: false, Label: "Data"}, MatchedPatterns: []value.MatchedPattern{}},
					},
				}}, offset: 0x4, size: 4, endian: "little"}, fl)
}

func TestEvaluateIfChildren(t *testing.T) {
	templateData := `
constants:
  u8 C1: "04"

structs:
  header:
    u8 Number: "04"

    if self.Number in (4):
      u8 Four: "ff"

    if self.Number notin (6):
      u8 NotSix: "aa"

    if self.Number in (C1):
      u8 FourConstant: "ee"

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		0x04, // Number
		0xff, // Four
		0xaa, // NotSix
		0xee, // FourConstant
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{
					Label: "Header",
					Fields: []Field{
						{Offset: 0x0, Length: 0x1, Value: []uint8{0x4}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Number"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x1, Length: 0x1, Value: []uint8{0xff}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Four"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x2, Length: 0x1, Value: []uint8{0xaa}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "NotSix"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x3, Length: 0x1, Value: []uint8{0xee}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "FourConstant"}, MatchedPatterns: []value.MatchedPattern{}},
					},
				}}, offset: 0x4, size: 4}, fl)
}

func TestEvaluateIfMulti(t *testing.T) {
	// simplified form of tiff type validation
	templateData := `
constants:
  ascii[2] BIG:    c'MM'
  ascii[2] LITTLE: c'II'

structs:
  header:
    ascii[2] Signature: ??

    if self.Signature in (BIG):
      endian: big
    if self.Signature in (LITTLE):
      endian: little

    if self.Signature notin (BIG, LITTLE):
      data: invalid

layout:
  - header Header
`

	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	test := []struct {
		in  []byte
		out *FileLayout
		err error
	}{
		// BIG
		{[]byte{'M', 'M'},
			&FileLayout{
				Structs: []Struct{
					{
						Label: "Header",
						Fields: []Field{
							{Offset: 0x0, Length: 0x2, Value: []uint8{'M', 'M'}, Endian: "", Format: value.DataField{Kind: "ascii", Range: "2", Slice: false, Label: "Signature"}, MatchedPatterns: []value.MatchedPattern{}},
						},
					}}, offset: 0x2, size: 2, endian: "big"},
			nil},

		// LITTLE
		{[]byte{'I', 'I'},
			&FileLayout{
				Structs: []Struct{
					{
						Label: "Header",
						Fields: []Field{
							{Offset: 0x0, Length: 0x2, Value: []uint8{'I', 'I'}, Endian: "", Format: value.DataField{Kind: "ascii", Range: "2", Slice: false, Label: "Signature"}, MatchedPatterns: []value.MatchedPattern{}},
						},
					}}, offset: 0x2, size: 2, endian: "little"},
			nil},

		// invalid
		{[]byte{'M', 'a'}, nil, fmt.Errorf("file invalidated by template")},
	}

	for _, tt := range test {
		fl, err := MapReader(bytes.NewReader(tt.in), ds)
		assert.Equal(t, tt.err, err)
		if tt.err == nil {
			assert.Equal(t, tt.out, fl)
		}
	}
}

func TestEvaluateIfBitfield(t *testing.T) {
	templateData := `
structs:
  header:
    u8 Bit field:
      bit b1000_0000: High bit

    if self.Bit field.High bit in (1):   # true if bitfield value is exactly 1
      u8 HighExact: "aa"

    if self.Bit field.High bit:          # true if bitfield value is non-zero
      u8 HighSet: "bb"

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		0xff, // Bit field
		0xaa, // HighExact
		0xbb, // HighSet
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{
					Label: "Header",
					Fields: []Field{
						{Offset: 0x0, Length: 0x1, Value: []uint8{0xff}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Bit field"},
							MatchedPatterns: []value.MatchedPattern{{Label: "High bit", Operation: "bit", Value: 1}}},
						{Offset: 0x1, Length: 0x1, Value: []uint8{0xaa}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "HighExact"},
							MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x2, Length: 0x1, Value: []uint8{0xbb}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "HighSet"},
							MatchedPatterns: []value.MatchedPattern{}},
					},
				}},
			offset: 0x3, size: 3}, fl)
}

func TestEvaluateAsciiz(t *testing.T) {
	templateData := `
structs:
  header:
    asciiz Name: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		'f', 'o', 'o', 0x00, // Name
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{
					Label: "Header",
					Fields: []Field{
						{Offset: 0x0, Length: 0x4, Value: []uint8{'f', 'o', 'o', 0x00}, Endian: "", Format: value.DataField{Kind: "asciiz", Range: "", Slice: false, Label: "Name"}},
					},
				}},
			offset: 0x4, size: 0x4}, fl)
}

func TestEvaluateUTF16LE(t *testing.T) {
	templateData := `
structs:
  header:
    endian: little
    utf16le[3] Name: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		'f', 0x00, 'o', 0x00, 'o', 0x00, // Name
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{
					Label:  "Header",
					Fields: []Field{{Offset: 0x0, Length: 0x6, Value: []uint8{0x00, 'f', 0x00, 'o', 0x00, 'o'}, Endian: "little", Format: value.DataField{Kind: "utf16le", Range: "3", Slice: false, Label: "Name"}, MatchedPatterns: []value.MatchedPattern{}}}}},
			endian: "little", offset: 0x6, size: 0x6}, fl)
}

func TestEvaluateStructSlice(t *testing.T) {
	templateData := `
structs:
  header:
    u8 ID: "02"
  block:
    u8 First: ??
    u8 Second: ??

layout:
  - header Header
  - block[] Unsized block
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		0x02,       // Size
		0x80, 0x81, // Block #1
		0x90, 0x91, // Block #2
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{
					Label: "Header",
					Fields: []Field{
						{Offset: 0x0, Length: 0x1, Value: []uint8{0x02}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "ID"}, MatchedPatterns: []value.MatchedPattern{}},
					},
				},

				{
					Label: "Unsized block[0]",
					Fields: []Field{
						{Offset: 0x1, Length: 0x1, Value: []uint8{0x80}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "First"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x2, Length: 0x1, Value: []uint8{0x81}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Second"}, MatchedPatterns: []value.MatchedPattern{}},
					},
				},

				{
					Label: "Unsized block[1]",
					Fields: []Field{
						{Offset: 0x3, Length: 0x1, Value: []uint8{0x90}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "First"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x4, Length: 0x1, Value: []uint8{0x91}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Second"}, MatchedPatterns: []value.MatchedPattern{}},
					},
				},
			},
			offset: 0x5, size: 0x5}, fl)
}

/*
func TestEvaluateAbsoluteArray(t *testing.T) {
	// tests that "u8[start:length]" syntax works
	templateData := `
structs:
  header:
    u8 Offset1: ??
    u8 Length1: ??
    u8 Offset2: ??
    u8 Length2: ??
    u8[self.Offset1:self.Length1] Data1: ??
    u8[self.Offset2:self.Length2] Data2: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		0x06, // Offset1
		0x03, // Length1

		0x0a, // Offset2
		0x02, // Length2

		0xb0, 0xb1, // unmapped padding
		0xf0, 0xf1, 0xf2, // Data1
		0xb2, // unmapped padding

		0xf3, 0xf4, // Data2
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	assert.Equal(t,
		&FileLayout{
			Structs: []Struct{
				{
					Label: "Header",
					Fields: []Field{
						{Offset: 0x0, Length: 0x1, Value: []uint8{0x6}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Offset1"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x1, Length: 0x1, Value: []uint8{0x3}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Length1"}, MatchedPatterns: []value.MatchedPattern{}},

						{Offset: 0x2, Length: 0x1, Value: []uint8{0xa}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Offset2"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0x3, Length: 0x1, Value: []uint8{0x2}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Length2"}, MatchedPatterns: []value.MatchedPattern{}},

						{Offset: 0x6, Length: 0x3, Value: []uint8{0xf0, 0xf1, 0xf2}, Endian: "", Format: value.DataField{Kind: "u8", Range: "6:3", Slice: false, Label: "Data1"}, MatchedPatterns: []value.MatchedPattern{}},
						{Offset: 0xa, Length: 0x2, Value: []uint8{0xf3, 0xf4}, Endian: "", Format: value.DataField{Kind: "u8", Range: "10:2", Slice: false, Label: "Data2"}, MatchedPatterns: []value.MatchedPattern{}},
					},
				},
			},
			offset: 0x4, size: 0xc}, fl)
}
*/
