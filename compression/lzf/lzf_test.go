package lzf

/*
func TestLzfDecompress(t *testing.T) {

	log.SetFlags(log.Lshortfile)

	compressed := []byte{
		0x04, 0x00, 0x00, 0x00, 0x1F, 0xFF, 0xFE, 0x3C, 0x00, 0x3F, 0x00, 0x78, 0x00, 0x6D, 0x00, 0x6C,
		0x00, 0x20, 0x00, 0x76, 0x00, 0x65, 0x00, 0x72, 0x00, 0x73, 0x00, 0x69, 0x00, 0x6F, 0x00, 0x6E,
		0x00, 0x3D, 0x00, 0x22, 0x00, 0x04, 0x31, 0x00, 0x2E, 0x00, 0x30, 0x20, 0x07, 0x00,
	}

	// expected := []byte{2} // XXX

	decompressed, err := Decompress(bytes.NewReader(compressed), 2)
	assert.Nil(t, err)
	spew.Dump(decompressed)
	//assert.Equal(t, expected, decompressed) // XXX
}
*/
