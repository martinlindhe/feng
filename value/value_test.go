package value

import (
	"fmt"
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
		{"b0000_0000 b0111_1111", []byte{0x00, 0x7f}},
		{"b0000_0000_0111_1111", []byte{0x00, 0x7f}},
	}

	for _, h := range test {
		res, err := ParseHexString(h.in)
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

		{"b00000000_01111111", "00 7f"},
		{"b00000001_01111111", "01 7f"},

		{"b00000000_00000000_01111111", "00 00 7f"},
		{"b00000001_00000010_01111111", "01 02 7f"},

		{"b00000000_00000000_00000000_01111111", "00 00 00 7f"},
		{"b00000001_00000010_00000011_01111111", "01 02 03 7f"},
	}

	for _, h := range test {
		res, err := replaceNextBitTag(h.in)
		assert.Equal(t, nil, err)
		assert.Equal(t, h.out, res)
	}
}

func TestParseDataField(t *testing.T) {

	test := []struct {
		field       string
		expected    *DataField
		expectedErr error
	}{
		{"u16 Width", &DataField{Kind: "u16", Range: "", Label: "Width"}, nil},
		{"u8[5] Label", &DataField{Kind: "u8", Range: "5", Label: "Label"}, nil},
		{"endian big", &DataField{Kind: "endian", Range: "", Label: "big"}, nil},
		{"offset self.offset+4", &DataField{Kind: "offset", Range: "", Label: "self.offset+4"}, nil},
		{"Seg[self.offset+4] My label", &DataField{Kind: "Seg", Range: "self.offset+4", Label: "My label"}, nil},

		// should fail
		{"Seg[self.offset]", &DataField{}, fmt.Errorf("token label missing")},
	}
	for _, h := range test {
		field, err := ParseDataField(h.field)
		assert.Equal(t, h.expectedErr, err)
		assert.Equal(t, field, *h.expected)
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

func TestDosTime(t *testing.T) {
	assert.Equal(t, "16:24:50", AsDosTime(33561).String())
}

func TestDosDate(t *testing.T) {
	assert.Equal(t, "2016-04-10", AsDosDate(18570).String())
}

func TestDosTimeDate(t *testing.T) {
	assert.Equal(t, "2016-03-17 01:36:40 +0000 UTC", AsDosTimeDate(1215368340).String())
}

func TestUtf16String(t *testing.T) {
	b := []byte{
		0x31, 0x00, 0x68, 0x00, 0x32, 0x00, 0x74, 0x00, 0x78, 0x00, 0x79, 0x00, 0x65, 0x00, 0x77, 0x00,
		0x79, 0x00, 0x5C, 0x00, 0x53, 0x00, 0x65, 0x00, 0x74, 0x00, 0x74, 0x00, 0x69, 0x00, 0x6E, 0x00,
		0x67, 0x00, 0x73, 0x00, 0x5C, 0x00, 0x73, 0x00, 0x65, 0x00, 0x74, 0x00, 0x74, 0x00, 0x69, 0x00,
		0x6E, 0x00, 0x67, 0x00, 0x73, 0x00, 0x2E, 0x00, 0x64, 0x00, 0x61, 0x00, 0x74, 0x00, 0x00, 0x00,
	}

	// convert to big endian for the test
	for i := 0; i < len(b); i += 2 {
		n1 := b[i]
		n2 := b[i+1]
		b[i] = n2
		b[i+1] = n1
	}
	assert.Equal(t, "1h2txyewy\\Settings\\settings.dat", Utf16String(b))
}

func TestUtf16zString(t *testing.T) {
	b := []byte{
		0x2E, 0x00, 0x65, 0x00, 0x78, 0x00, 0x65, 0x00, 0x00, 0x00,
		0x31, 0x00, // trailing data that should be ignored
	}
	assert.Equal(t, ".exe", Utf16zString(b))
}

func TestUtf8zString(t *testing.T) {
	b := []byte{
		0xc2, 0xa1, 0x45, 0x68, 0x21, 0x00,
		0x31, 0x00, // trailing data that should be ignored
	}
	assert.Equal(t, "¡Eh!", Utf8zString(b))
}
