package mapper

import (
	"bytes"
	"testing"

	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateExpression(t *testing.T) {

	fl := &FileLayout{
		Structs: []Struct{
			{
				Label: "Header",
				Fields: []Field{
					{Offset: 0x0, Length: 0x1, Value: []uint8{0x6}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Val1"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x1, Length: 0x1, Value: []uint8{0x3}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Len1"}, MatchedPatterns: []value.MatchedPattern{}},
				},
			},
		},
		offset: 0x4, size: 0xc}

	test := []struct {
		expr     string
		expected uint64
	}{
		{"4+2", 6},
		{"2 << 10", 0x800},
		{"abs(-44)", 44},
		{"Header.Val1 * 2", 12},

		//        u8[1 << self.Field.Size] Data: ??

		//templates\images\gif.yml : cant evaluate '3 * (2 << self.Screen descriptor.Global Color Table size)': var error: variable "self" does not exist
	}
	for _, h := range test {
		a, err := fl.EvaluateExpression(h.expr)
		assert.Nil(t, err)
		assert.Equal(t, h.expected, a)
	}
}

func TestCalcArraySize(t *testing.T) {
	// based on calc used in archives/rar.yml
	templateData := `
endian: little
structs:
  header:
    u16 CRC: ??
    u16 HeaderSize: ??
    u8[self.HeaderSize - (self.offset - offset("self.CRC"))] Reserved: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	data := []byte{
		// Header
		0x02, 0x01, // CRC
		0x08, 0x00, // HeaderSize
		0xf0, 0xf1, 0xf2, 0xf3, // Reserved
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	// Header
	_, val, err := fl.GetValue("Header.Reserved", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, 4, len(val), val)
}
