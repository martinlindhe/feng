package compression

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func mockFile(t *testing.T, filename string, data []byte) afero.File {
	appFS := afero.NewMemMapFs()
	afero.WriteFile(appFS, filename, data, 0644)
	f, err := appFS.Open(filename)
	assert.Nil(t, err)
	return f
}

func TestExtractors(t *testing.T) {

	tests := []struct {
		comp       string
		raw        []byte
		compressed []byte
	}{
		{
			comp:       "deflate",
			raw:        []byte{'h', 'e', 'l', 'l', 'o'},
			compressed: []byte{0xca, 0x48, 0xcd, 0xc9, 0xc9, 0x07, 0x04, 0x00, 0x00, 0xff, 0xff},
		},
		{
			comp:       "deflate",
			raw:        []byte{},
			compressed: []byte{0x01, 0x00, 0x00, 0xff, 0xff},
		},

		{
			comp:       "zlib",
			raw:        []byte{'h', 'e', 'l', 'l', 'o'},
			compressed: []byte{0x78, 0xda, 0xcb, 0x48, 0xcd, 0xc9, 0xc9, 0x07, 0x00, 0x06, 0x2c, 0x02, 0x15},
		},
		{
			comp:       "zlib",
			raw:        []byte{},
			compressed: []byte{0x78, 0x9c, 0x01, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00, 0x00, 0x01},
		},

		{
			comp: "gzip",
			raw:  []byte{'h', 'e', 'l', 'l', 'o'},
			compressed: []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xca, 0x48, 0xcd, 0xc9, 0xc9, 0x07,
				0x04, 0x00, 0x00, 0xff, 0xff, 0x86, 0xa6, 0x10, 0x36, 0x05, 0x00, 0x00, 0x00},
		},

		{
			comp: "lzma",
			raw:  []byte{'h', 'e', 'l', 'l', 'o'},
			compressed: []byte{0x5d, 0x00, 0x00, 0x80, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00, 0x34, 0x19,
				0x49, 0xee, 0x8e, 0x68, 0x21, 0xff, 0xff, 0xff, 0xb9, 0xe0, 0x00, 0x00},
		},

		{
			comp: "lz4",
			raw:  []byte{'h', 'e', 'l', 'l', 'o'},
			compressed: []byte{0x04, 0x22, 0x4d, 0x18, 0x64, 0x70, 0xb9, 0x05, 0x00, 0x00, 0x80, 0x68, 0x65, 0x6c, 0x6c, 0x6f,
				0x00, 0x00, 0x00, 0x00, 0xf9, 0x77, 0x00, 0xfb},
		},

		{
			comp:       "lzo1",
			raw:        []byte{'h', 'e', 'l', 'l', 'o'},
			compressed: []byte{0x16, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x11, 0x00, 0x00},
		},
		{
			comp:       "lzo1",
			raw:        []byte{},
			compressed: []byte{0x11, 0x0, 0x0},
		},
	}

	for idx, tt := range tests {
		extractor, err := ExtractorFactory(tt.comp)
		assert.Nil(t, err, idx)

		// test that the compression is not failing
		out := new(bytes.Buffer)

		err = extractor.Compress(tt.raw, out)
		assert.Nil(t, err, idx)

		comp := out.Bytes()
		fmt.Printf("compressed %s as %s into % 02x\n", tt.raw, tt.comp, comp)

		// test that a known compressed stream decompresses to the same input
		f := mockFile(t, "in", tt.compressed)
		b, err := extractor.Extract(f)
		assert.Nil(t, err, idx)
		assert.Equal(t, tt.raw, b, idx)

		// test that the compressed data decompress back to the same input
		f = mockFile(t, "in", comp)
		b, err = extractor.Extract(f)
		assert.Nil(t, err, idx)
		assert.Equal(t, tt.raw, b, idx)
	}
}
