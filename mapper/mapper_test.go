package mapper

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/martinlindhe/feng/template"
)

func TestMapReaderDataTypes(t *testing.T) {
	templateData := `
structs:
  header:
    u8 U8 single: fb
    u8[2] U8 array: ff d8

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
	}

	r := bytes.NewReader(data)

	ff, err := MapReader(r, ds)
	assert.Equal(t, nil, err)

	assert.Equal(t, []fileField{
		{Offset: 0x0, Length: 0x1, Value: []byte{0xfb}},
		{Offset: 0x1, Length: 0x2, Value: []byte{0xff, 0xd8}},

		// big endian
		{Offset: 0x3, Length: 0x2, Value: []byte{0xaa, 0xf0}},
		{Offset: 0x5, Length: 0x4, Value: []byte{0x10, 0x11, 0x20, 0x21}},
		{Offset: 0x9, Length: 0x4, Value: []uint8{0x11, 0x22, 0x33, 0x44}},
		{Offset: 0xd, Length: 0x8, Value: []uint8{0x10, 0x11, 0x12, 0x13, 0x20, 0x21, 0x22, 0x23}},
		{Offset: 0x15, Length: 0x8, Value: []uint8{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}},
		{Offset: 0x1d, Length: 0x10, Value: []uint8{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27}},

		// little endian
		{Offset: 0x2d, Length: 0x2, Value: []byte{0xaa, 0xf0}},
		{Offset: 0x2f, Length: 0x4, Value: []byte{0x10, 0x11, 0x20, 0x21}},
		{Offset: 0x33, Length: 0x4, Value: []uint8{0x11, 0x22, 0x33, 0x44}},
		{Offset: 0x37, Length: 0x8, Value: []uint8{0x10, 0x11, 0x12, 0x13, 0x20, 0x21, 0x22, 0x23}},
		{Offset: 0x3f, Length: 0x8, Value: []uint8{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}},
		{Offset: 0x47, Length: 0x10, Value: []uint8{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27}},
	}, ff)
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
    #u64 U64BE single: "1122334455667788"
    #u64[2] U64BE array: 1011121314151617 2021222324252627

    #endian: little
    #u16 U16LE single: aaf0
    #u16[2] U16LE array: 1011 2021
    #u32 U32LE single: "11223344"
    #u32[2] U32LE array: 10111213 20212223
    #u64 U64LE single: "1122334455667788"
    #u64[2] U64LE array: 1011121314151617 2021222324252627

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	test := []struct {
		in  []byte
		out []fileField
		err error
	}{
		{[]byte{}, []fileField{}, io.EOF},

		{[]byte{
			// expected bytes
			0xfb, 0xff, 0xd8, // u8 single, u8[2] array
			0x4a, 0xf0, // u16 single
			0x16, 0x17, 0x16, 0x17, // u16[2] array
			0x12, 0x34, 0x56, 0x78, // u32 single
			0x22, 0x22, 0x33, 0x33, 0x44, 0x44, 0x55, 0x55, // u32[2] array
		}, []fileField{
			{Offset: 0x0, Length: 0x1, Value: []byte{0xfb}},
			{Offset: 0x1, Length: 0x2, Value: []byte{0xff, 0xd8}},
			{Offset: 0x3, Length: 0x2, Value: []byte{0x4a, 0xf0}},
			{Offset: 0x5, Length: 0x4, Value: []byte{0x16, 0x17, 0x16, 0x17}},
			{Offset: 0x9, Length: 0x4, Value: []byte{0x12, 0x34, 0x56, 0x78}},
			{Offset: 0xd, Length: 0x8, Value: []byte{0x22, 0x22, 0x33, 0x33, 0x44, 0x44, 0x55, 0x55}},
		}, nil},

		{[]byte{
			// wrong first byte
			0xfa, 0xff, 0xd8, // u8 single,u8[2] array
		}, []fileField{}, errors.New("[00000000] pattern 'Sig8a' does not match. expected 'fb', got 'fa'")},

		{[]byte{
			// wrong second byte
			0xfb, 0xfe, 0xd8, // u8 single, u8[2] array
		}, []fileField{}, errors.New("[00000001] pattern 'Sig8b' does not match. expected 'ff d8', got 'fe d8'")},

		{[]byte{
			// wrong third byte
			0xfb, 0xff, 0xdd, // u8 single, u8[2] array
		}, []fileField{}, errors.New("[00000001] pattern 'Sig8b' does not match. expected 'ff d8', got 'ff dd'")},
	}

	for _, tt := range test {
		r := bytes.NewReader(tt.in)
		ff, err := MapReader(r, ds)
		assert.Equal(t, tt.err, err)
		if tt.err == nil {
			assert.Equal(t, tt.out, ff)
		}
	}
}
