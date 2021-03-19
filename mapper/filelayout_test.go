package mapper

import (
	"testing"

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
