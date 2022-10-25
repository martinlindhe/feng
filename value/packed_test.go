package value

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadVariableLengthU32(t *testing.T) {
	tests := []struct {
		Bytes          []byte
		ExpectedLength uint64
		ExpectedValue  uint32
	}{
		{[]byte{0x3f}, 1, 0x3f},
		{[]byte{0xa9, 0x24}, 2, 5284},
		{[]byte{0x81, 0xe5, 0x65}, 3, 29413},

		// from bpg:
		{[]byte{0x08}, 1, 8},
		{[]byte{0x84, 0x1e}, 2, 542},
		{[]byte{0xac, 0xbe, 0x17}, 3, 728855},
	}
	for _, tst := range tests {
		r := bytes.NewReader(tst.Bytes)
		got, _, len, err := ReadVariableLengthU32(r)
		assert.Equal(t, nil, err)
		assert.Equal(t, tst.ExpectedLength, len)
		assert.Equal(t, tst.ExpectedValue, got)
	}
}
func TestReadVariableLengthU64(t *testing.T) {
	tests := []struct {
		Bytes          []byte
		ExpectedLength uint64
		ExpectedValue  uint64
	}{
		{[]byte{0x3f}, 1, 0x3f},
		{[]byte{0xa9, 0x24}, 2, 5284},
		{[]byte{0x81, 0xe5, 0x65}, 3, 29413},
	}
	for _, tst := range tests {
		r := bytes.NewReader(tst.Bytes)
		got, _, len, err := ReadVariableLengthU64(r)
		assert.Equal(t, nil, err)
		assert.Equal(t, tst.ExpectedLength, len)
		assert.Equal(t, tst.ExpectedValue, got)
	}
}

func TestReadVariableLengthS64(t *testing.T) {
	tests := []struct {
		Bytes          []byte
		ExpectedLength uint64
		ExpectedValue  uint64
	}{
		{[]byte{0x8c}, 1, 0xc},
	}
	for _, tst := range tests {
		r := bytes.NewReader(tst.Bytes)
		got, _, len, err := ReadVariableLengthS64(r)
		assert.Equal(t, nil, err)
		assert.Equal(t, tst.ExpectedLength, len)
		assert.Equal(t, tst.ExpectedValue, got)
	}
}
