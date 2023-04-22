package compression

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
	"github.com/rasky/go-lzo"
	"github.com/spf13/afero"
	"github.com/ulikunitz/xz/lzma"
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
	case "gzip":
		return Gzip{}, nil
	case "deflate":
		return Deflate{}, nil
	case "lzma":
		return Lzma{}, nil
	case "lzo1":
		return Lzo1x{}, nil
	case "lz4":
		return Lz4{}, nil
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

/*
// TODO need go rework github.com/zhuyie/golzf to support reader interface, or finish lzf port in compression/lzf/lzf.go
///replace github.com/zhuyie/golzf v0.0.0-20161112031142-8387b0307ade => ../golzf

type Lzf struct{}

func (o Lzf) Extract(f afero.File) ([]byte, error) {
	expanded := make([]byte, 1024*1024) // XXX have a "known" expanded size value ready from format parsing
	n, err := lzf.Decompress(data, expanded)
	return expanded[0:n], err
}

func (o Lzf) Compress(in []byte, w io.Writer) error {
	buf := make([]byte, len(in)-1)
	n, err := lzf.Compress(in, buf)
	w.Write(buf[:n])
	return err
}
*/

/*
// TODO need github.com/fbonhomm/LZSS to support reader interface

/// https://github.com/fbonhomm/LZSS/pull/1
///replace github.com/fbonhomm/LZSS v0.0.0-20200907090355-ba1a01a92989 => github.com/martinlindhe/LZSS v0.0.0-20221025204446-acc47c959dfe

type Lzss struct{}

func (o Lzss) Extract(f afero.File) ([]byte, error) {

	lzssMode0 := lzss.LZSS{}
	expanded := lzssMode0.Decompress(data)
	return expanded, nil
}

func (o Lzss) Compress(in []byte, w io.Writer) error {
	lzssMode0 := lzss.LZSS{}
	_, err := w.Write(lzssMode0.Compress(in))
	return err
}
*/
