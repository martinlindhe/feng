package value

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	bitExpressionRE = regexp.MustCompile(`b([01_]+)`)
)

// parses a "structs" data value (used by structs parser)
func ParseDataPattern(in string) (DataPattern, error) {
	dp := DataPattern{}
	if in == "??" {
		return dp, nil
	}
	value, err := ParseHexString(in)
	if err == nil {
		dp.Pattern = value
		dp.Known = true
	} else {
		dp.Value = in
	}
	//log.Debug().Msgf("ParseDataPattern: '%s' => '%s'", in, value)
	return dp, nil
}

type DataPattern struct {
	// if false, data value was described as "??"
	Known bool

	// if true, holds the expected pattern
	Pattern []byte

	// holds the value (for custom values like in "endian: big")
	Value string
}

func ParseHexStringToUint64(in string) (uint64, error) {
	b, err := ParseHexString(in)
	if err != nil {
		return 0, err
	}
	return AsUint64Raw(b), nil
}

// parses a textual representation of data into a byte array
func ParseHexString(in string) ([]byte, error) {

	var err error
	prev := ""
	for {
		prev = in
		in, err = replaceNextASCIITag(in)
		if err != nil {
			return nil, err
		}
		in, err = replaceNextBitTag(in)
		if err != nil {
			return nil, err
		}

		if in == prev {
			break
		}
	}

	in = strings.ReplaceAll(in, " ", "")
	in = strings.ReplaceAll(in, "_", "")
	res, err := hex.DecodeString(in)
	if err != nil {
		return nil, fmt.Errorf("hex decode '%s' failed: %v", in, err)
	}
	return res, nil
}

// find next b1010_1010 and replace with hex
func replaceNextBitTag(s string) (string, error) {

	matches := bitExpressionRE.FindStringSubmatch(s)
	if len(matches) == 0 {
		return s, nil
	}

	idx := bitExpressionRE.FindStringSubmatchIndex(s)

	m := strings.ReplaceAll(matches[1], "_", "")
	i, err := strconv.ParseUint(m, 2, 64)
	if err != nil {
		return "", err
	}

	//log.Debug().Msgf("replaceNextBitTag(%s): %d", m, i)

	// XXX fix bit sizing
	lm := len(m)
	res := ""
	switch {
	case lm <= 8: // u8
		res = fmt.Sprintf("%02x", i)

	case lm <= 16: // u16
		x := U64toBytesBigEndian(i, 2)
		res = fmt.Sprintf("% 02x", x)

	case lm <= 24: // 3 bytes
		x := U64toBytesBigEndian(i, 3)
		res = fmt.Sprintf("% 02x", x)

	case lm <= 32: // u32
		x := U64toBytesBigEndian(i, 4)
		res = fmt.Sprintf("% 02x", x)

	case lm <= 64: // u64
		x := U64toBytesBigEndian(i, 8)
		res = fmt.Sprintf("% 02x", x)

	default:
		log.Fatal().Msgf("unhandled bit length %d", lm)
	}

	s = s[0:idx[0]] + res + s[idx[1]:]

	return s, nil
}

func U64toBytesBigEndian(val uint64, unitSize uint64) []byte {
	r := make([]byte, 8)
	for i := uint64(0); i < 8; i++ {
		r[7-i] = byte((val >> (i * 8)) & 0xff)
	}

	switch unitSize {
	case 1:
		return r[7:8]
	case 2:
		return r[6:8]
	case 3:
		return r[5:8]
	case 4:
		return r[4:8]
	case 8:
		return r
	default:
		log.Fatal().Msgf("unhandled unit size %d", unitSize)
	}
	return r
}

// find next ascii between c” characters and replace with hex
func replaceNextASCIITag(s string) (string, error) {
	p1 := strings.Index(s, "c'")
	if p1 == -1 {
		return s, nil
	}
	block := s[p1+len("c'"):]
	p2 := strings.Index(block, "'")
	if p2 == -1 {
		return "", fmt.Errorf("missing closing ' delimiter")
	}
	block = block[:p2]

	tmp, err := asciiToHexString(block)
	if err != nil {
		return "", err
	}
	s = s[0:p1] + tmp + s[p1+len("c'")+p2+1:]
	return s, nil
}

func asciiToHexString(s string) (string, error) {
	res := ""
	for _, r := range s {
		v := int(r)
		if v < 0 || v > 127 {
			return "", fmt.Errorf("unexpected rune value (not ascii) '%v'", r)
		}
		//fmt.Printf("%c => ascii %02x\n", r, v)
		res += fmt.Sprintf("%02x", v)
	}
	return res, nil
}

func ParseDataField(in string) (DataField, error) {

	df := DataField{}

	p1 := strings.Index(in, "[")
	p2 := strings.Index(in, "]")
	if p2 < p1 {
		return df, fmt.Errorf("invalid range syntax '%s'", in)
	}

	// slice format: kind[] label
	if p2 == p1+1 {
		df.Slice = true
	}

	space := strings.Index(in, " ")
	if space == -1 {
		if in == "label" || in == "parse" || in == "endian" || in == "else" || in == "data" || in == "filename" || in == "offset" || in == "until" || in == "import" || in == "default" || in == "xor_key" {
			// single token like "endian"
			df.Kind = in
			return df, nil
		}

		return df, fmt.Errorf("token label missing in '%s", in)
	}

	if p1 >= 0 {
		// ranged format: kind[range] label
		df.Kind, df.Endian = parseKindEndianPair(in[0:p1])
		df.Range = strings.TrimSpace(in[p1+1 : p2])
		df.Label = strings.TrimSpace(in[p2+1:])
	} else {
		// non-ranged format: kind label
		df.Kind, df.Endian = parseKindEndianPair(in[0:space])
		df.Label = strings.TrimSpace(in[space+1:])
	}

	return df, nil
}

// parses "u8" or "be:datetime". returns kind, endian
func parseKindEndianPair(s string) (string, string) {
	s = strings.TrimSpace(s)
	idx := strings.Index(s, ":")
	if idx == -1 {
		return s, ""
	}
	endian := s[0:idx]
	kind := s[idx+1:]
	switch endian {
	case "le":
		endian = "little"
	case "be":
		endian = "big"
	default:
		// not a endian + datatype pair, do nothing
		return s, ""
	}
	return kind, endian
}

// a parsed data field
type DataField struct {
	// data type: u8, ascii etc
	Kind string

	// ranged value if set. data field is type Kind[Length] or Kind[Start:Length]
	Range string

	// XXX make this unexported
	RangeVal int64

	// a slice type if true. data field is type Kind[]
	Slice bool

	// field label
	Label string

	// tracks the index of this DataField in it's parent array
	Index int

	// if set, endianness override for this field
	Endian string
}

// A match for values of a fileField.
// Based on template.MatchPattern data from file template.
type MatchedPattern struct {
	Label string

	// for debugging
	Operation string

	// parsed value of bit field
	Value []byte

	// parsed value for display
	Parsed string

	// size of bitfield, in bits
	Size int8

	// index bit
	Index int8
}

func (df *DataField) SingleUnitSize() int64 {
	return SingleUnitSize(df.Kind)
}

func SingleUnitSize(kind string) int64 {
	switch kind {
	case "u8", "i8",

		// all string types are counted in bytes
		"ascii", "asciiz", "asciinl",
		"utf8z",
		"utf16", "utf16z",
		"sjis",

		// all variable size number types are counted in bytes
		"vs64",

		// all compressed types are counted in bytes
		"compressed:lzo1x",
		"compressed:lz4",
		"compressed:lzf",
		"compressed:lzss",
		"compressed:zlib",
		"compressed:zlib_loose",
		"compressed:gzip",
		"compressed:lzma",
		"compressed:lzma2",
		"compressed:deflate",
		"compressed:pkware",
		"raw:u8", "encrypted:u8":
		return 1
	case "u16", "i16",
		"dostime", "dosdate":
		return 2
	case "u24", "rgb8":
		return 3
	case "u32", "i32", "f32", "time_t_32", "dostimedate":
		return 4
	case "u64", "i64", "filetime":
		return 8
	case "rgba32", "xyzm32":
		return 16
	}
	panic(fmt.Sprintf("SingleUnitSize cant handle kind '%s'", kind))
}

// returns true if unit is a single u8, u16, u32 or u64 that can have eq/bit field as child
func (df *DataField) IsPatternableUnit() bool {
	return df.IsSimpleUnit()
}

// returns true if unit is a single u8, u16, u32 or u64 that can be used in IF statements
func (df *DataField) IsSimpleUnit() bool {
	if df.Slice || df.Range != "" {
		return false
	}
	switch df.Kind {
	case "u8", "i8", "u16", "i16", "i24", "u24", "u32", "i32", "f32", "u64", "i64", "ascii":
		return true
	}
	return false
}

// reverse byte order in groups of `unitLength` to handle u16/u32 ordering
func ReverseBytes(b []byte, unitLength int) []byte {

	if len(b)%unitLength != 0 {
		log.Fatal().Msgf("invalid input '%v' (len %d), unitLength %d", b, len(b), unitLength)
	}

	res := make([]byte, len(b))
	for i := 0; i < len(b); i += unitLength {
		for j := 0; j < unitLength; j++ {
			res[i+unitLength-1-j] = b[i+j]
		}
	}

	return res
}

// decodes value in network byte order (big) to unsigned integer
func AsUint64(kind string, b []byte) uint64 {

	log.Debug().Msgf("AsUint64 converting [%02x] to %s", b, kind)

	switch kind {
	case "u8", "i8", "ascii":
		return uint64(b[0])
	case "u16", "i16", "dosdate", "dostime":
		return uint64(binary.BigEndian.Uint16(b))
	case "u24", "i24":
		four := append([]byte{0}, b...)
		return uint64(binary.BigEndian.Uint32(four))
	case "u32", "i32", "f32", "time_t_32":
		return uint64(binary.BigEndian.Uint32(b))
	case "u64", "i64":
		return binary.BigEndian.Uint64(b)
	}
	log.Fatal().Msgf("AsUint64 unhandled kind %s", kind)
	return 0
}

// decodes value in network byte order (big) to unsigned integer
func AsUint64Raw(b []byte) (v uint64) {

	if len(b) == 1 {
		v = uint64(b[0])
	} else if len(b) == 2 {
		v = uint64(binary.BigEndian.Uint16(b))
	} else if len(b) == 3 {
		d := append([]byte{0}, b...)
		v = uint64(binary.BigEndian.Uint32(d))
	} else if len(b) == 4 {
		v = uint64(binary.BigEndian.Uint32(b))
	} else {
		v = binary.BigEndian.Uint64(b)
	}

	//log.Debug().Msgf("AsUint6Raw converting [%02x] to %d", b, v)

	return
}

// decodes value in network byte order (big) to signed integer
func AsInt64(kind string, b []byte) int64 {

	//log.Debug().Msgf("AsInt64 converting [%02x] to %s", b, kind)

	switch kind {
	case "i8":
		return int64(int8(b[0]))
	case "i16":
		return int64(int16(binary.BigEndian.Uint16(b)))
	case "i32":
		return int64(int32(binary.BigEndian.Uint32(b)))
	case "i64":
		return int64(binary.BigEndian.Uint64(b))
	}
	log.Fatal().Msgf("AsInt64 unhandled kind %s", kind)
	return 0
}

// text encoding used by Windows
func Utf16String(b []byte) string {

	// Make an transformer that converts MS-Win default to UTF8
	win16be := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)

	// Make a transformer that is like win16be, but abides by BOM
	utf16bom := unicode.BOMOverride(win16be.NewDecoder())

	unicodeReader := transform.NewReader(bytes.NewReader(b), utf16bom)

	decoded, err := io.ReadAll(unicodeReader)
	if err != nil {
		panic(err)
	}
	return strings.TrimRight(string(decoded), "\x00")
}

// text encoding used by Windows (00 00-terminated)
func Utf8zString(b []byte) string {
	end := 0
	for i := 0; i < len(b); i++ {
		end = i
		if b[i] == 0 {
			break
		}
	}
	return string(b[:end])
}

// text encoding used by Windows (00 00-terminated)
func Utf16zString(b []byte) string {
	end := 0
	// read 2 bytes until eof or 00
	for i := 0; i < len(b); i += 2 {
		v := binary.LittleEndian.Uint16(b[i:])
		end = i
		if v == 0 {
			break
		}
	}

	// Make an transformer that converts MS-Win default to UTF8
	win16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM) // XXX used for xbox xbe string decode

	// Make a transformer that is like win16be, but abides by BOM
	utf16bom := unicode.BOMOverride(win16le.NewDecoder())

	unicodeReader := transform.NewReader(bytes.NewReader(b[0:end]), utf16bom)

	decoded, err := io.ReadAll(unicodeReader)
	if err != nil {
		panic(err)
	}
	return string(decoded)
}

// decodes ShiftJIS into a utf8 string
func ShiftJISString(b []byte) string {
	decoder := transform.NewReader(bytes.NewReader(b), japanese.ShiftJIS.NewDecoder())
	decBytes, _ := io.ReadAll(decoder)
	return string(decBytes)
}

// ascii text encoding (00-terminated)
// returns decoded string and length in bytes
func AsciiZString(b []byte, maxLength int) (string, uint64) {
	length := uint64(0)
	decoded := ""
	for _, v := range b {
		length++
		if v == 0 {
			break
		}
		if v >= 0x20 && v < 0x7f {
			decoded += string(v)
		} else {
			decoded += "·"
		}
		if maxLength > 0 && length >= uint64(maxLength) {
			break
		}
	}
	return decoded, length
}

// ascii text encoding (non-printable is replaced by "·")
// returns decoded string and length in bytes
func AsciiPrintableString(b []byte, maxLength int) (string, uint64) {
	length := uint64(0)
	decoded := ""
	for _, v := range b {
		length++
		if v >= 0x20 && v < 0x7f {
			decoded += string(v)
		} else {
			decoded += "·"
		}
		if maxLength > 0 && length >= uint64(maxLength) {
			break
		}
	}
	return decoded, length
}
