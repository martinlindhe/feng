package value

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDataPattern(t *testing.T) {

	test := []struct {
		in  string
		out []byte
	}{
		{"1f 8b", []byte{0x1f, 0x8b}},
		{"1f8b", []byte{0x1f, 0x8b}},
		{"89 c'PNG'", []byte{0x89, 0x50, 0x4e, 0x47}},
		{"c'IHDR'", []byte{'I', 'H', 'D', 'R'}},
		{"12 c'B' 13 c'C'", []byte{0x12, 'B', 0x13, 'C'}},
		{"b0000_0001", []byte{0x01}},
	}

	for _, h := range test {
		res, err := ParseDataString(h.in)
		assert.Equal(t, nil, err)
		assert.Equal(t, h.out, res)
	}
}

func TestReplaceNextBitTag(t *testing.T) {
	test := []struct {
		in  string
		out string
	}{
		{"b0000_0001", "01"},
		{"b0000_0001 ff", "01 ff"},
		{"ff b0000_1111 fe", "ff 0f fe"},
		{"ff b1111_0000 fe", "ff f0 fe"},
		{"fa b1111_0000", "fa f0"},
	}

	for _, h := range test {
		res, err := replaceNextBitTag(h.in)
		assert.Equal(t, nil, err)
		assert.Equal(t, h.out, res)
	}
}

func TestParseDataField(t *testing.T) {

	test := []struct {
		field    string
		expected DataField
	}{
		{"u16 Width", DataField{Kind: "u16", Range: "", Label: "Width"}},
		{"u8[5] Label", DataField{Kind: "u8", Range: "5", Label: "Label"}},
		{"endian big", DataField{Kind: "endian", Range: "", Label: "big"}},
		{"Seg[self.offset+4] My label", DataField{Kind: "Seg", Range: "self.offset+4", Label: "My label"}},
	}

	for _, h := range test {
		field, err := ParseDataField(h.field)
		assert.Equal(t, nil, err)
		assert.Equal(t, field, h.expected)
	}
}

func TestReverseBytes(t *testing.T) {
	assert.Equal(t, []byte{0x44}, ReverseBytes([]byte{0x44}, 1))

	// u16
	assert.Equal(t, []byte{0x20, 0x44}, ReverseBytes([]byte{0x44, 0x20}, 2))
	assert.Equal(t, []byte{0x20, 0x44, 0x21, 0x45}, ReverseBytes([]byte{0x44, 0x20, 0x45, 0x21}, 2))

	// u32
	assert.Equal(t, []byte{0x11, 0x22, 0x33, 0x44}, ReverseBytes([]byte{0x44, 0x33, 0x22, 0x11}, 4))
	assert.Equal(t, []byte{0x11, 0x22, 0x33, 0x44, 0x10, 0x21, 0x32, 0x43}, ReverseBytes([]byte{0x44, 0x33, 0x22, 0x11, 0x43, 0x32, 0x21, 0x10}, 4))
}
