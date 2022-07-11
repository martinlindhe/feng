package value

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	DEBUG = false

	// TODO: make IN_UTC configurable from cli
	IN_UTC = true
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
	if DEBUG {
		log.Printf("ParseDataPattern: '%s' => '%s'", in, value)
	}
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

	if DEBUG {
		log.Printf("replaceNextBitTag(%s): %d", m, i)
	}

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
		log.Fatalf("unhandled bit length %d", lm)
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
		log.Fatalf("unhandled unit size %d", unitSize)
	}
	return r
}

// find next ascii between c'' characters and replace with hex
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
		// single token like "endian"
		df.Kind = in
	} else if p1 >= 0 {
		// ranged format: kind[range] label
		df.Kind = strings.TrimSpace(in[0:p1])
		df.Range = strings.TrimSpace(in[p1+1 : p2])
		df.Label = strings.TrimSpace(in[p2+1:])
	} else {
		// non-ranged format: kind label
		df.Kind = strings.TrimSpace(in[0:space])
		df.Label = strings.TrimSpace(in[space+1:])
	}

	return df, nil
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
}

// A match for values of a fileField.
// Based on template.MatchPattern data from file template.
type MatchedPattern struct {
	Label string

	// for debugging
	Operation string

	// parsed value of bit field
	Value uint64

	// parsed value for display
	Parsed string

	// size of bitfield, in bits
	Size int8

	// index bit
	Index int8
}

func (df *DataField) SingleUnitSize() uint64 {
	return SingleUnitSize(df.Kind)
}

func SingleUnitSize(kind string) uint64 {
	switch kind {
	case "u8", "i8", "ascii", "asciiz",
		"compressed:lz4",
		"compressed:zlib",
		"compressed:deflate",
		"raw:u8":
		return 1
	case "u16", "i16", "utf16", "utf16z",
		"dostime", "dosdate":
		return 2
	case "u32", "i32", "time_t_32":
		return 4
	case "u64", "i64", "filetime":
		return 8
	}
	log.Fatalf("SingleUnitSize cant handle kind '%s'", kind)
	return 0
}

// returns true if unit is a single u8, u16, u32 or u64 that can have eq/bit field as child
func (df *DataField) IsPatternableUnit() bool {
	if df.Slice || df.Range != "" {
		return false
	}
	switch df.Kind {
	case "u8", "i8", "u16", "i16", "u32", "i32", "u64", "i64", "ascii":
		return true
	}
	return false
}

// returns true if unit is a single u8, u16, u32 or u64 that can be used in IF statements
func (df *DataField) IsSimpleUnit() bool {
	return df.IsPatternableUnit()
}

// reverse byte order in groups of `unitLength` to handle u16/u32 ordering
func ReverseBytes(b []byte, unitLength int) []byte {

	if len(b)%unitLength != 0 {
		log.Fatalf("invalid input '%v', length %d", b, unitLength)
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

	if DEBUG {
		log.Printf("AsUint64 converting [%02x] to %s", b, kind)
	}
	switch kind {
	case "u8", "i8", "ascii":
		return uint64(b[0])
	case "u16", "i16", "dosdate", "dostime":
		return uint64(binary.BigEndian.Uint16(b))
	case "u32", "i32", "time_t_32":
		return uint64(binary.BigEndian.Uint32(b))
	case "u64", "i64":
		return binary.BigEndian.Uint64(b)
	}
	log.Fatalf("AsUint64 unhandled kind %s", kind)
	return 0
}

// decodes value in network byte order (big) to unsigned integer
func AsUint64Raw(b []byte) (v uint64) {

	if len(b) == 1 {
		v = uint64(b[0])
	} else if len(b) <= 2 {
		v = uint64(binary.BigEndian.Uint16(b))
	} else if len(b) <= 4 {
		v = uint64(binary.BigEndian.Uint32(b))
	} else {
		v = binary.BigEndian.Uint64(b)
	}
	if DEBUG {
		//log.Printf("AsUint6Raw converting [%02x] to %d", b, v)
	}
	return
}

// decodes value in network byte order (big) to signed integer
func AsInt64(kind string, b []byte) int64 {
	if DEBUG {
		//log.Printf("AsInt64 converting [%02x] to %s", b, kind)
	}
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
	log.Fatalf("AsInt64 unhandled kind %s", kind)
	return 0
}

// presents the value of the data type (format.Kind) in a human-readable form
func (format DataField) Present(b []byte) string {
	switch format.Kind {
	case "compressed:deflate", "compressed:lz4", "compressed:zlib", "raw:u8":
		return ""

	case "u8", "u16", "u32", "u64":
		if format.Slice || format.Range != "" {
			return ""
		}
		return fmt.Sprintf("%d", AsUint64Raw(b))

	case "i8", "i16", "i32", "i64":
		if format.Slice || format.Range != "" {
			return ""
		}
		return fmt.Sprintf("%d", AsUint64Raw(b))

	case "ascii", "asciiz":
		v, _ := AsciiZString(b, len(b))
		return v

	case "utf16":
		return utf16String(b)

	case "utf16z":
		return utf16zString(b)

	case "time_t_32":
		v := AsUint64Raw(b)
		timestamp := time.Unix(int64(v), 0)
		if IN_UTC {
			timestamp = timestamp.UTC()
		}
		return timestamp.Format(time.RFC3339)

	case "filetime":
		ft := &syscall.Filetime{
			HighDateTime: binary.BigEndian.Uint32(b[:4]),
			LowDateTime:  binary.BigEndian.Uint32(b[4:]),
		}
		timestamp := time.Unix(0, ft.Nanoseconds())
		if IN_UTC {
			timestamp = timestamp.UTC()
		}
		return timestamp.Format(time.RFC3339)

	case "dostime":
		v := AsUint64Raw(b)
		return asDosTime(uint16(v)).String()

	case "dosdate":
		v := AsUint64Raw(b)
		return asDosDate(uint16(v)).String()
	}

	log.Fatalf("don't know how to present %s (slice:%v, range:%s): %v", format.Kind, format.Slice, format.Range, b)
	return ""
}

var (
	red = color.New(color.FgRed).SprintFunc()
)

// text encoding used by Windows
func utf16String(b []byte) string {

	// Make an transformer that converts MS-Win default to UTF8
	win16be := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)

	// Make a transformer that is like win16be, but abides by BOM
	utf16bom := unicode.BOMOverride(win16be.NewDecoder())

	unicodeReader := transform.NewReader(bytes.NewReader(b), utf16bom)

	decoded, err := ioutil.ReadAll(unicodeReader)
	if err != nil {
		panic(err)
	}
	return strings.TrimRight(string(decoded), "\x00")
}

// text encoding used by Windows (00 00-terminated)
func utf16zString(b []byte) string {
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

	decoded, err := ioutil.ReadAll(unicodeReader)
	if err != nil {
		panic(err)
	}
	return string(decoded)
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
			decoded += "."
		}
		if maxLength > 0 && length >= uint64(maxLength) {
			break
		}
	}
	return decoded, length
}
