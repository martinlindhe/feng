package mapper

import (
	"bytes"
	"testing"

	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
	"github.com/stretchr/testify/assert"
)

func TestFieldPresent(t *testing.T) {

	test := []struct {
		field    Field
		expected string
	}{
		{Field{Offset: 0x1, Length: 0x2, Value: []uint8{0xff, 0xd8}, Endian: "", Format: value.DataField{Kind: "u8", Range: "2", Slice: false, Label: "U8 array"}}, "  [000001] U8 array                       u8[2]                 ff d8               \n"},
	}
	for _, h := range test {
		assert.Equal(t, h.expected, h.field.Present())
	}
}

func TestGetValue(t *testing.T) {
	templateData := `
structs:
  header:
    u8 Number: ??
    u8 Field:
      bit b1000_0000: High bit

layout:
  - header Header
  - header Header2
`

	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	data := []byte{
		0x04, 0xff, // Header
		0x05, 0x00, // Header2
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

	_, val, err = fl.GetValue("Header.Field.High bit", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{1}, val)

	// Header2
	_, val, err = fl.GetValue("Header2.Number", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{5}, val)

	_, val, err = fl.GetValue("self.Number", &ds.Layout[1])
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{5}, val)
}
