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
		{[]byte{0x3f}, 1, 63},
		{[]byte{0xa9, 0x24}, 2, 5284},
		{[]byte{0x81, 0xe5, 0x65}, 3, 29413},
	}
	for _, tst := range tests {
		r := bytes.NewReader(tst.Bytes)
		got, _, len, err := ReadVariableLengthU32(r)
		assert.Equal(t, nil, err)
		assert.Equal(t, tst.ExpectedLength, len)
		assert.Equal(t, tst.ExpectedValue, got)
	}
}
