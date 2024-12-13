package compression

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/JoshVarga/blast"
	lzss "github.com/fbonhomm/LZSS/source"
	"github.com/pierrec/lz4/v4"
	"github.com/rasky/go-lzo"
	"github.com/spf13/afero"
	"github.com/ulikunitz/xz/lzma"

	"github.com/martinlindhe/feng/compression/lzf"
)

// The Extractor handles compression and decompression for a specific compression format
type Extractor interface {
	// Extracts compressed data from input stream `f`.
	Extract(f afero.File) ([]byte, error)

	// Compresses `in` data.
	Compress(in []byte, w io.Writer) error
}

func ExtractorFactory(name string) (Extractor, error) {
	switch name {
	case "zlib":
		return Zlib{}, nil
	case "zlib_loose":
		return ZlibLoose{}, nil
	case "gzip":
		return Gzip{}, nil
	case "deflate":
		return Deflate{}, nil
	case "lzma":
		return Lzma{}, nil
	case "lzma2":
		return Lzma2{}, nil
	case "lzo1x":
		return Lzo1x{}, nil
	case "lz4":
		return Lz4{}, nil
	case "lzf":
		return Lzf{}, nil
	case "lzss":
		return Lzss{}, nil
	case "pkware":
		return Pkware{}, nil
	}
	panic(fmt.Sprintf("unknown extractor '%s'", name))
}

type Deflate struct{}

func (o Deflate) Extract(f afero.File) ([]byte, error) {
	reader := flate.NewReader(f)
	defer reader.Close()
	out := new(bytes.Buffer)
	_, err := io.Copy(out, reader)
	return out.Bytes(), err
}

func (o Deflate) Compress(in []byte, w io.Writer) error {
	zw, err := flate.NewWriter(w, flate.BestCompression)
	if err != nil {
		return err
	}
	_, err = zw.Write(in)
	zw.Close()
	return err
}

type Zlib struct{}

func (o Zlib) Extract(f afero.File) ([]byte, error) {
	reader, err := zlib.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	out := new(bytes.Buffer)
	_, err = io.Copy(out, reader)
	return out.Bytes(), err
}

func (o Zlib) Compress(in []byte, w io.Writer) error {
	zw := zlib.NewWriter(w)
	_, err := zw.Write(in)
	zw.Close()
	return err
}

// ignores compression errors
type ZlibLoose struct{}

func (o ZlibLoose) Extract(f afero.File) ([]byte, error) {
	reader, err := zlib.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	out := new(bytes.Buffer)
	_, err = io.Copy(out, reader)
	return out.Bytes(), err
}

func (o ZlibLoose) Compress(in []byte, w io.Writer) error {
	zw := zlib.NewWriter(w)
	_, err := zw.Write(in)
	zw.Close()
	return err
}

type Gzip struct{}

func (o Gzip) Extract(f afero.File) ([]byte, error) {
	reader, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	out := new(bytes.Buffer)
	_, err = io.Copy(out, reader)
	return out.Bytes(), err
}

func (o Gzip) Compress(in []byte, w io.Writer) error {
	zw := gzip.NewWriter(w)
	_, err := zw.Write(in)
	zw.Close()
	return err
}

type Lzma struct{}

func (o Lzma) Extract(f afero.File) ([]byte, error) {
	reader, err := lzma.NewReader(f)
	if err != nil {
		return nil, err
	}

	out := new(bytes.Buffer)
	_, err = io.Copy(out, reader)
	return out.Bytes(), err
}

func (o Lzma) Compress(in []byte, w io.Writer) error {
	zw, err := lzma.NewWriter(w)
	if err != nil {
		return err
	}
	_, err = zw.Write(in)
	zw.Close()
	return err
}

type Lzma2 struct{}

func (o Lzma2) Extract(f afero.File) ([]byte, error) {
	reader, err := lzma.NewReader2(f)
	if err != nil {
		return nil, err
	}

	out := new(bytes.Buffer)
	_, err = io.Copy(out, reader)
	return out.Bytes(), err
}

func (o Lzma2) Compress(in []byte, w io.Writer) error {
	zw, err := lzma.NewWriter2(w)
	if err != nil {
		return err
	}
	_, err = zw.Write(in)
	zw.Close()
	return err
}

type Lzo1x struct{}

func (o Lzo1x) Extract(f afero.File) ([]byte, error) {
	b, err := lzo.Decompress1X(f, 0, 0)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (o Lzo1x) Compress(in []byte, w io.Writer) error {
	_, err := w.Write(lzo.Compress1X(in))
	return err
}

type Lz4 struct{}

func (o Lz4) Extract(f afero.File) ([]byte, error) {
	out := new(bytes.Buffer)
	reader := lz4.NewReader(f)

	_, err := io.Copy(out, reader)
	return out.Bytes(), err
}

func (o Lz4) Compress(in []byte, w io.Writer) error {
	zw := lz4.NewWriter(w)
	_, err := zw.Write(in)
	zw.Close()
	return err
}

type Lzf struct {
	CompressedSize uint // deduced from field size
}

func (o Lzf) Extract(f afero.File) ([]byte, error) {
	uncompressed, err := lzf.Decompress(f, o.CompressedSize)
	return uncompressed, err
}

func (o Lzf) Compress(in []byte, w io.Writer) error {
	panic("lzf compression TODO")
	/*
		buf := make([]byte, len(in)-1)
		n, err := lzf.Compress(in, buf)
		w.Write(buf[:n])
		return err
	*/
}

// LZSS is not really working ???, need a better impl.
type Lzss struct {
	CompressedSize uint // deduced from field size
}

func (o Lzss) Extract(f afero.File) ([]byte, error) {

	// TODO need github.com/fbonhomm/LZSS to support reader interface
	// https://github.com/fbonhomm/LZSS/pull/1

	data := make([]byte, o.CompressedSize)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, err
	}

	lzssMode := lzss.LZSS{Mode: 1, PositionMode: 1}
	return lzssMode.Decompress(data)
}

func (o Lzss) Compress(in []byte, w io.Writer) error {
	lzssMode := lzss.LZSS{Mode: 1, PositionMode: 0}
	_, err := w.Write(lzssMode.Compress(in))
	return err
}

// PKWARE DCL compressed data (aka blast/explode/implode)
type Pkware struct{}

func (o Pkware) Extract(f afero.File) ([]byte, error) {
	out := new(bytes.Buffer)
	r, err := blast.NewReader(f)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(out, r)
	r.Close()
	return out.Bytes(), err
}

func (o Pkware) Compress(in []byte, w io.Writer) error {
	var b bytes.Buffer
	z := blast.NewWriter(&b, blast.Binary, blast.DictionarySize1024)
	_, err := z.Write(in)
	z.Close()
	if err != nil {
		return err
	}
	_, err = w.Write(b.Bytes())
	return err
}
