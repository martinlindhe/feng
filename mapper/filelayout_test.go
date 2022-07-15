package mapper

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
	"github.com/stretchr/testify/assert"
)

func TestFieldPresent(t *testing.T) {

	fl := &FileLayout{}

	test := []struct {
		expected string
		field    Field
	}{
		{"  [000000] U8 array                       u8[2]                                  ff d8\n", Field{Length: 0x2, Value: []uint8{0xff, 0xd8}, Endian: "", Format: value.DataField{Kind: "u8", Range: "2", Slice: false, Label: "U8 array"}}},
		{"  [000000] TimeT_32                       time_t_32 le     2013-10-16T10:09:51Z  52 5e 65 ef\n", Field{Length: 0x4, Value: []uint8{0x52, 0x5e, 0x65, 0xef}, Endian: "little", Format: value.DataField{Kind: "time_t_32", Range: "", Slice: false, Label: "TimeT_32"}}},

		{"  [000000] FileTime                       filetime le      2017-01-17T08:12:02Z  32 2b 4f 62 99 70 d2 01\n", Field{Length: 0x8, Value: []uint8{0x32, 0x2b, 0x4f, 0x62, 0x99, 0x70, 0xd2, 0x01}, Endian: "little", Format: value.DataField{Kind: "filetime", Range: "", Slice: false, Label: "FileTime"}}},

		{"  [000000] color                          rgb8 le          (1, 2, 3)             01 02 03\n", Field{Length: 0x3, Value: []uint8{0x01, 0x02, 0x03}, Endian: "little", Format: value.DataField{Kind: "rgb8", Range: "", Slice: false, Label: "color"}}},

		// XXX test "utf16 be"  and "utf16 le"
		{"  [000000] UTF16                          utf16[3] le      foo                   00 66 00 6f 00 6f\n", Field{Length: 0x6, Value: []uint8{0x00, 'f', 0x00, 'o', 0x00, 'o'}, Endian: "little", Format: value.DataField{Kind: "utf16", Range: "3", Slice: false, Label: "UTF16"}}},

		{"  [000000] u8                             u8               255                   ff\n", Field{Length: 0x1, Value: []uint8{0xff}, Endian: "", Format: value.DataField{Kind: "u8", Slice: false, Label: "u8"}}},
		{"  [000000] i8                             i8               -1                    ff\n", Field{Length: 0x1, Value: []uint8{0xff}, Endian: "", Format: value.DataField{Kind: "i8", Slice: false, Label: "i8"}}},

		{"  [000000] Signed                         i16 le           -1                    ff ff\n", Field{Length: 0x2, Value: []uint8{0xff, 0xff}, Endian: "little", Format: value.DataField{Kind: "i16", Slice: false, Label: "Signed"}}},
		{"  [000000] Signed                         i32 le           -1                    ff ff ff ff\n", Field{Length: 0x4, Value: []uint8{0xff, 0xff, 0xff, 0xff}, Endian: "little", Format: value.DataField{Kind: "i32", Slice: false, Label: "Signed"}}},
		{"  [000000] Signed                         i64 le           -1                    ff ff ff ff ff ff ff ff\n", Field{Length: 0x8, Value: []uint8{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, Endian: "little", Format: value.DataField{Kind: "i64", Slice: false, Label: "Signed"}}},
	}
	for _, h := range test {
		assert.Equal(t, h.expected, fl.presentField(&h.field, true))
	}
}

func TestGetValue(t *testing.T) {
	templateData := `
structs:
  header:
    u8 Number: ??
    u8 Field:
      bit b1000_0000: High bit
    if self.Number in (5):
      u8[FILE_SIZE-self.offset] Extra: ??   # refers to current field offset

layout:
  - header Header
  - header Header2
`

	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	data := []byte{
		// Header
		0x04, // Number
		0xff, // Field

		// Header2
		0x05,       // Number
		0x00,       // Field
		0xbb, 0xaa, // Extra
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	// Header
	_, val, err := fl.GetValue("Header.Number", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{4}, val)

	_, val, err = fl.GetValue("self.Number", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{4}, val)

	_, val, err = fl.GetValue("self.Field.High bit", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{1}, val)

	_, _, err = fl.GetValue("self.Extra", &ds.Layout[0])
	assert.Equal(t, fmt.Errorf("GetValue: 'Header.Extra' not found"), err)

	// Header2
	_, val, err = fl.GetValue("Header2.Number", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{5}, val)

	_, val, err = fl.GetValue("self.Number", &ds.Layout[1])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{5}, val)

	// assert that "u8[FILE_SIZE-self.offset]" evaluates to u8[6-4] == u8[2] (all remaining bytes)
	assert.Equal(t, "  [000004] Extra                          u8[2]                                  bb aa\n", fl.presentField(&fl.Structs[1].Fields[2], false))
	_, val, err = fl.GetValue("self.Extra", &ds.Layout[1])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{0xbb, 0xaa}, val)
}

func TestGetFieldOffset(t *testing.T) {
	templateData := `
structs:
  header:
    u8[2] Number: ??
    u8 ID: ??
    u8[self.ID.offset] Padding: ??          # refers to a previous field

layout:
  - header Header
`

	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	data := []byte{
		// Header
		0x04, 0x05, // Number
		0x07,       // ID
		0xff, 0xfe, // Padding

	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	// Header
	_, val, err := fl.GetValue("Header.Number", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{0x04, 0x05}, val)

	_, val, err = fl.GetValue("self.Padding", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{0xff, 0xfe}, val)
}

/*
func TestGetFieldLen(t *testing.T) {
	templateData := `
structs:
  header:
    u8[3] Number: ??
    u8[self.Number.len] Pad1: ??                        # refers to a previous field
    u8[self.Pad1.offset - self.Pad1.len + 1] Pad2: ??   # expression with multiple variables    3 - 3 + 1 = 1

layout:
  - header Header
`

	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		// Header
		0x04, 0x05, 0x06, // Number
		0xff, 0xfe, 0xfd, // Pad1
		0xaa, // Pad2
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	// Header
	_, val, err := fl.GetValue("Header.Number", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{0x04, 0x05, 0x06}, val)

	_, val, err = fl.GetValue("self.Pad1", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{0xff, 0xfe, 0xfd}, val)

	_, val, err = fl.GetValue("self.Pad2", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{0xaa}, val)
}
*/

func TestDataFieldPresentType(t *testing.T) {

	fl := FileLayout{}

	test := []struct {
		field    value.DataField
		expected string
	}{
		{value.DataField{Kind: "u32", Range: "2"}, "u32[2]"},
		{value.DataField{Kind: "u8", Range: "2"}, "u8[2]"},
		{value.DataField{Kind: "u32"}, "u32"},
	}
	for _, h := range test {
		assert.Equal(t, h.expected, fl.PresentType(&h.field))
	}
}
