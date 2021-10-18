package mapper

import (
	"testing"

	"github.com/martinlindhe/feng/value"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateExpression(t *testing.T) {

	fl := &FileLayout{
		Structs: []Struct{
			{
				Label: "Header",
				Fields: []Field{
					{Offset: 0x0, Length: 0x1, Value: []uint8{0x6}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Offset"}, MatchedPatterns: []value.MatchedPattern{}},
					{Offset: 0x1, Length: 0x1, Value: []uint8{0x3}, Endian: "", Format: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Length"}, MatchedPatterns: []value.MatchedPattern{}},
				},
			},
		},
		offset: 0x4, size: 0xc}

	test := []struct {
		expr     string
		expected uint64
	}{
		{"4+2", 6},
		{"abs(-44)", 44},
		{"Header.Offset + 2", 8}, // should become 6 + 2
	}
	for _, h := range test {
		a, err := fl.EvaluateExpression(h.expr)
		assert.Nil(t, err)
		assert.Equal(t, h.expected, a)
	}
}
