package mapper

import (
	"crypto/aes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/maja42/goval"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"

	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

// parsed file data from a template "layout"
type FileLayout struct {
	Structs []*Struct

	// pointer to its internal yaml representation, so we can access "constants"
	DS *template.DataStructure

	// current endian ("big", "little")
	endian string

	// filename for the current output data
	filename string

	// adjust the starting position with this value
	startOffset int64

	// current offset
	offset int64

	// previous offset (restore it with "offset: restore")
	previousOffsets []int64

	// counts how many times the offset was changed in order to stop recursion
	offsetChanges int64

	// total size of data (FILE_SIZE)
	size int64

	// default extension
	Extension string

	// lastpath/filename-without-ext, eg "archives/zip"
	BaseName string

	// if unseen, ask user to submit a sample
	unseen bool

	// present datetimes in UTC
	inUTC bool

	// current encryption method
	encryptionMethod string

	// current encryption key
	encryptionKey []byte

	// file handle
	_f afero.File

	// bytes read, for debugging over-reading
	bytesRead int

	// bytes imported from external files, for debugging over-reading
	bytesImported int

	// number of times fl.EvaluateExpressions() was called during parsing
	evaluatedExpressions int

	// time spent evaluating expressions during parsing
	evaluatedExpressionTime time.Duration

	// debugging: measure evaluation time?
	measureTime bool

	// time spent evaluating the template
	evaluationTime time.Duration

	// time spent presenting the template
	presentTime time.Duration

	// time spent evaluating templates until match was found, including this template
	totalEvaluationTimeUntilMatch time.Duration

	scriptFunctions map[string]goval.ExpressionFunction

	scriptVariables map[string]interface{}

	eval *goval.Evaluator
}

// pop last offset from previousOffsets list
func (fl *FileLayout) popLastOffset() (v int64) {
	if len(fl.previousOffsets) == 0 {
		panic("cannot pop offset, no offsets have been pushed")
	}

	v, fl.previousOffsets = fl.previousOffsets[len(fl.previousOffsets)-1], fl.previousOffsets[:len(fl.previousOffsets)-1]
	return
}

// push current offset to previousOffsets list
func (fl *FileLayout) pushOffset() int64 {
	fl.previousOffsets = append(fl.previousOffsets, fl.offset)
	return fl.offset
}

func (fl *FileLayout) DecryptData(encData []byte) ([]byte, error) {
	switch fl.encryptionMethod {
	case "aes_128_cbc":
		return decryptCBC(fl.encryptionKey, encData)
	}
	return nil, fmt.Errorf("unknown encryption method '%s'", fl.encryptionMethod)
}

func decryptCBC(key, cipherText []byte) (plaintext []byte, err error) {

	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	decrypted := make([]byte, len(cipherText))
	c.Decrypt(decrypted, cipherText)

	/*

		iv := make([]byte, 16) // 0:s

		//if len(cipherText)%aes.BlockSize != 0 {
		//	panic("cipherText is not a multiple of the block size")
		//}

		decrypted := make([]byte, aes.BlockSize+len(cipherText))

		mode := cipher.NewCBCDecrypter(block, iv)
		mode.CryptBlocks(decrypted, cipherText)
	*/
	return cipherText, nil

}

// parsed file data section from a template "struct"
type Struct struct {
	// unique name of this instance of the struct
	Name string

	// child nodes
	Children []*Struct

	// additional decoration
	Label string

	Fields []Field

	// slice-based counter index, 0-based
	Index int

	// when evaluated once, struct can be skipped by eval function
	evaluated bool

	//evaluatedMap map[string]interface{}
}

type Field struct {
	Offset int64
	Length int64

	// on-disk endianness
	Endian string

	// underlying data structure
	Format value.DataField

	// matched patterns
	MatchedPatterns []value.MatchedPattern

	// filename for the next output data
	Outfile string

	// if set, data is imported from external file
	ImportFile string
}

var (
	variableExpressionRE = regexp.MustCompile(`([\w .+\-*/()<>"&]+)`)
)

const (
	maxHexDisplayLength = 16
)

func prettyFloat(f float32) string {
	if f == 1. {
		return "1."
	}
	if f == 0. {
		return "0."
	}
	if f == -1. {
		return "-1."
	}
	return fmt.Sprintf("%.04f", f)
}

// returns a presentation of the value in the data type (field.Format.Kind)
func (fl *FileLayout) GetFieldValue(field *Field) interface{} {
	switch field.Format.Kind {
	case "compressed:deflate", "compressed:lzo1x", "compressed:lzss", "compressed:lz4",
		"compressed:lzf", "compressed:zlib", "compressed:zlib_loose", "compressed:gzip",
		"compressed:lzma", "compressed:lzma2",
		"raw:u8", "encrypted:u8":
		return ""
	}

	b, err := fl.peekBytes(field)
	if err != nil {
		panic(err)
	}

	switch field.Format.Kind {
	case "f32":
		if field.Format.Slice || field.Format.Range != "" {
			return ""
		}
		return prettyFloat(math.Float32frombits(uint32(value.AsUint64Raw(b))))

	case "xyzm32":
		if field.Format.Slice || field.Format.Range != "" {
			return ""
		}
		return fmt.Sprintf("[%s, %s, %s, %s]",
			prettyFloat(math.Float32frombits(uint32(value.AsUint64Raw(b[:4])))),
			prettyFloat(math.Float32frombits(uint32(value.AsUint64Raw(b[4:8])))),
			prettyFloat(math.Float32frombits(uint32(value.AsUint64Raw(b[8:12])))),
			prettyFloat(math.Float32frombits(uint32(value.AsUint64Raw(b[12:16])))))

	case "u8", "u16", "u24", "u32", "u64", "i8", "i16", "i32", "i64":
		if !field.Format.Slice && field.Format.Range != "" {
			log.Debug().Msgf("GetFieldValue %s", field.Format.Label)
			unitLength, totalLength := fl.GetAddressLengthPair(&field.Format)
			values := []interface{}{}
			switch field.Format.Kind {
			case "u8", "i8":
				return b[0]

			case "u16":
				for i := int64(0); i < totalLength; i += unitLength {
					if field.Endian == "big" {
						values = append(values, uint16(binary.BigEndian.Uint16(b[i:])))
					} else {
						values = append(values, uint16(binary.LittleEndian.Uint16(b[i:])))
					}
				}
			case "u32":
				for i := int64(0); i < totalLength; i += unitLength {
					if field.Endian == "big" {
						values = append(values, uint32(binary.BigEndian.Uint32(b[i:])))
					} else {
						values = append(values, uint32(binary.LittleEndian.Uint32(b[i:])))
					}
				}
			case "u64":
				for i := int64(0); i < totalLength; i += unitLength {
					if field.Endian == "big" {
						values = append(values, binary.BigEndian.Uint64(b[i:]))
					} else {
						values = append(values, binary.LittleEndian.Uint64(b[i:]))
					}
				}
			case "i32":
				for i := int64(0); i < totalLength; i += unitLength {
					if field.Endian == "big" {
						values = append(values, int32(binary.BigEndian.Uint32(b[i:])))
					} else {
						values = append(values, int32(binary.LittleEndian.Uint32(b[i:])))
					}
				}
			default:
				panic("handle " + field.Format.Kind)
			}
			return values
		}

		return int(value.AsUint64Raw(b))

	case "ascii", "asciinl":
		v, _ := value.AsciiPrintableString(b, len(b))
		return v

	case "asciiz":
		v, _ := value.AsciiZString(b, len(b))
		return v

	case "utf8z":
		return value.Utf8zString(b)

	case "utf16":
		return value.Utf16String(b)

	case "utf16z":
		return value.Utf16zString(b)

	case "sjis":
		return value.ShiftJISString(b)

	case "time_t_32":
		v := value.AsUint64Raw(b)
		timestamp := time.Unix(int64(v), 0)
		if fl.inUTC {
			timestamp = timestamp.UTC()
		}
		return timestamp.Format(time.RFC3339)

	case "filetime":
		timestamp := value.AsFileTime(b)
		if fl.inUTC {
			timestamp = timestamp.UTC()
		}
		return timestamp.Format(time.RFC3339)

	case "dostime":
		v := value.AsUint64Raw(b)
		return value.AsDosTime(uint16(v)).String()

	case "dosdate":
		v := value.AsUint64Raw(b)
		return value.AsDosDate(uint16(v)).String()

	case "dostimedate":
		v := value.AsUint64Raw(b)
		timestamp := value.AsDosTimeDate(uint32(v))
		if fl.inUTC {
			timestamp = timestamp.UTC()
		}
		return timestamp.Format(time.RFC3339)

	case "rgb8":
		return fmt.Sprintf("[%d, %d, %d]", b[0], b[1], b[2])

	case "rgba32":
		return fmt.Sprintf("[%d, %d, %d, %d]",
			uint32(value.AsUint64Raw(b[:4])),
			uint32(value.AsUint64Raw(b[4:8])),
			uint32(value.AsUint64Raw(b[8:12])),
			uint32(value.AsUint64Raw(b[12:16])))

	case "vu32":
		got, _, _, _ := fl.ReadVariableLengthU32()
		return got

	case "vu64":
		got, _, _, _ := fl.ReadVariableLengthU64()
		return got

	case "vs64":
		got, _, _, _ := fl.ReadVariableLengthS64()
		return got
	}

	log.Fatal().Msgf("GetFieldValue: unhandled %s (slice:%v, range:%s): %v", field.Format.Kind, field.Format.Slice, field.Format.Range, b)
	return ""
}

// presents the value of the data type (field.Format.Kind) in a human-readable form
func (fl *FileLayout) PresentFieldValue(field *Field, b []byte) string {

	switch field.Format.Kind {
	case "compressed:deflate", "compressed:lzo1x", "compressed:lzss", "compressed:lz4",
		"compressed:lzf", "compressed:zlib", "compressed:zlib_loose", "compressed:gzip",
		"compressed:lzma", "compressed:lzma2",
		"raw:u8", "encrypted:u8":
		return ""
	}

	switch field.Format.Kind {
	case "u8", "u16", "u24", "u32", "u64", "i8", "i16", "i32", "i64", "f32":
		if !field.Format.Slice && field.Format.Range != "" {
			unitLength, totalLength := fl.GetAddressLengthPair(&field.Format)

			values := []string{}
			val := 0
			skipRest := false

			for i := int64(0); i < totalLength; i += unitLength {
				val++

				switch field.Format.Kind {

				case "u8":
					values = append(values, fmt.Sprintf("%d", b[i]))
					if i+unitLength < totalLength && val >= 5 {
						skipRest = true
					}

				case "u16":
					if field.Endian == "big" {
						values = append(values, fmt.Sprintf("%d", binary.BigEndian.Uint16(b[i:])))
					} else {
						values = append(values, fmt.Sprintf("%d", binary.LittleEndian.Uint16(b[i:])))
					}
					if i+unitLength < totalLength && val >= 3 {
						skipRest = true
					}

				case "u32":
					if field.Endian == "big" {
						values = append(values, fmt.Sprintf("%d", binary.BigEndian.Uint32(b[i:])))
					} else {
						values = append(values, fmt.Sprintf("%d", binary.LittleEndian.Uint32(b[i:])))
					}
					if i+unitLength < totalLength && val >= 3 {
						skipRest = true
					}

				case "u64":
					if field.Endian == "big" {
						values = append(values, fmt.Sprintf("%d", binary.BigEndian.Uint64(b[i:])))
					} else {
						values = append(values, fmt.Sprintf("%d", binary.LittleEndian.Uint64(b[i:])))
					}
					if i+unitLength < totalLength && val >= 3 {
						skipRest = true
					}

				case "i8":
					values = append(values, fmt.Sprintf("%d", int8(b[i])))
					if i+unitLength < totalLength && val >= 4 {
						skipRest = true
					}

				case "i16":
					if field.Endian == "big" {
						values = append(values, fmt.Sprintf("%d", int16(binary.BigEndian.Uint16(b[i:]))))
					} else {
						values = append(values, fmt.Sprintf("%d", int16(binary.LittleEndian.Uint16(b[i:]))))
					}
					if i+unitLength < totalLength && val >= 3 {
						skipRest = true
					}

				case "i32":
					if field.Endian == "big" {
						values = append(values, fmt.Sprintf("%d", int32(binary.BigEndian.Uint32(b[i:]))))
					} else {
						values = append(values, fmt.Sprintf("%d", int32(binary.LittleEndian.Uint32(b[i:]))))
					}
					if i+unitLength < totalLength && val >= 3 {
						skipRest = true
					}

				case "i64":
					if field.Endian == "big" {
						values = append(values, fmt.Sprintf("%d", int64(binary.BigEndian.Uint64(b[i:]))))
					} else {
						values = append(values, fmt.Sprintf("%d", int64(binary.LittleEndian.Uint64(b[i:]))))
					}
					if i+unitLength < totalLength && val >= 3 {
						skipRest = true
						break
					}

				case "f32":
					if field.Endian == "big" {
						values = append(values, prettyFloat(math.Float32frombits(binary.BigEndian.Uint32(b[i:]))))
					} else {
						values = append(values, prettyFloat(math.Float32frombits(binary.LittleEndian.Uint32(b[i:]))))
					}
					if val >= 3 {
						skipRest = true
					}

				default:
					panic("FIXME handle " + field.Format.Kind)
				}
				if skipRest {
					break
				}
			}

			if skipRest {
				return "[" + strings.Join(values, ", ") + " ... ]"
			}
			return "[" + strings.Join(values, ", ") + "]"
		}
		switch field.Format.Kind {
		case "i8":
			return fmt.Sprintf("%d", int8(value.AsUint64Raw(b)))
		case "i16":
			return fmt.Sprintf("%d", int16(value.AsUint64Raw(b)))
		case "i32":
			return fmt.Sprintf("%d", int32(value.AsUint64Raw(b)))
		case "i64":
			return fmt.Sprintf("%d", int64(value.AsUint64Raw(b)))
		case "f32":
			return prettyFloat(math.Float32frombits(uint32(value.AsUint64Raw(b))))
		default:
			return fmt.Sprintf("%d", value.AsUint64Raw(b))
		}

	case "ascii", "asciiz", "asciinl", "xyzm32", "utf16", "utf8z", "utf16z", "sjis", "time_t_32", "filetime", "dostime", "dosdate", "dostimedate",
		"rgb8", "rgba32":
		res := fl.GetFieldValue(field).(string)
		if len(res) > 100 {
			res = res[0:100] // XXX cut better, respect rune widths
			return presentStringValue(res) + "..."
		}
		return presentStringValue(res)

	case "vu64", "vs64":
		res := fl.GetFieldValue(field).(uint64)
		return fmt.Sprintf("%d", res)
	}

	log.Fatal().Msgf("don't know how to present %s (slice:%v, range:%s): %v", field.Format.Kind, field.Format.Slice, field.Format.Range, b)
	return ""
}

// renders lines of ascii to present the data field for humans
func (fl *FileLayout) presentField(field *Field, cfg *PresentFileLayoutConfig) string {

	kind := fl.PresentType(&field.Format)
	if (field.Format.Kind != "vu32" && field.Format.Kind != "vu64") && field.Format.SingleUnitSize() > 1 {
		// XXX hacky way of skipping variable length fields
		if field.Endian == "big" {
			kind += " be"
		} else {
			kind += " le"
		}
	}

	maxLen := int64(maxHexDisplayLength)
	if field.Length < maxLen {
		maxLen = field.Length
	}

	var data []byte
	var err error

	if field.ImportFile != "" {
		log.Info().Msgf("IMPORT PEEK: reading %d bytes from %06x in %s", maxLen, field.Offset, field.ImportFile)

		f, err := os.Open(field.ImportFile) // XXX use afero
		if err != nil {
			panic(err)
		}
		defer f.Close()

		_, err = f.Seek(field.Offset, io.SeekStart)
		if err != nil {
			panic(err)
		}

		data = make([]byte, maxLen)
		n, err := f.Read(data)
		fl.bytesImported += n
		if err != nil {
			panic(err)
		}
	} else {

		data, err = fl.peekBytesMainFile(int64(field.Offset), maxLen)
		if err != nil {
			panic(err)
		}
	}

	// convert to network byte order
	unitLength, _ := fl.GetAddressLengthPair(&field.Format)
	if unitLength > 1 && field.Endian == "little" {
		data = value.ReverseBytes(data, int(unitLength))
	}
	fieldValue := strings.TrimRight(fl.PresentFieldValue(field, data), " ")

	res := ""
	if !cfg.ShowInDecimal {
		res = fmt.Sprintf("  [%06x] ", field.Offset)
	} else {
		res = fmt.Sprintf("  [%06d] ", field.Offset)
	}
	if !cfg.ShowRaw {
		res += fmt.Sprintf("%-30s %-16s %-30s",
			field.Format.Label, kind, fieldValue)
	} else {
		hexValue := ""
		if field.Length <= maxHexDisplayLength {
			hexValue = fmt.Sprintf("% 02x", data)
		} else {
			hexValue = fmt.Sprintf("% 02x ...", data[0:maxHexDisplayLength])
		}
		res += fmt.Sprintf("%-30s %-16s %-30s %-20s",
			field.Format.Label, kind, fieldValue, hexValue)
	}
	res = strings.TrimRight(res, " ") + "\n"

	for _, child := range field.MatchedPatterns {
		op := ""
		pretty := ""
		if child.Operation == "bit" {
			// decorate bit range
			op = fmt.Sprintf("bit %d:%d", child.Index, child.Size)
			pretty = child.Parsed
		} else {
			op = child.Operation
		}

		line := fmt.Sprintf("           - %-28s %-16s %-21s", child.Label, op, pretty)
		res += strings.TrimRight(line, " ") + "\n"
	}
	return res
}

func (fl *FileLayout) PresentStructureTree(structs []*Struct) {
	feng.Printf("# structure tree of %s\n", fl._f.Name())

	for _, layout := range structs {
		fl.presentStructureTreeNode(layout, 0)
	}
}
func (fl *FileLayout) presentStructureTreeNode(layout *Struct, indent int) {
	prefix := strings.Repeat(" ", indent)

	heading := prefix + layout.Name
	if layout.Label != "" {
		heading += ` "` + layout.Label + `"`
	}
	if len(layout.Fields) == 0 {
		feng.Printf("   empty struct")
	}
	feng.Printf(heading + "\n")
	for _, child := range layout.Children {
		feng.Printf(prefix)
		fl.presentStructureTreeNode(child, indent+2)
	}
}

type PresentFileLayoutConfig struct {
	ShowRaw           bool
	ShowInDecimal     bool
	ReportUnmapped    bool
	ReportOverlapping bool
	InUTC             bool
}

func (fl *FileLayout) presentStruct(layout *Struct, cfg *PresentFileLayoutConfig) string {
	if len(layout.Fields) == 0 {
		log.Debug().Msgf("skip empty struct '%s'\n", layout.Name)
		return ""
	}
	heading := layout.Name
	if layout.Label != "" {
		heading += " \"" + layout.Label + "\""
	}
	res := heading + "\n"
	for _, field := range layout.Fields {
		res += fl.presentField(&field, cfg)
	}
	res += "\n"

	for _, child := range layout.Children {
		res += fl.presentStruct(child, cfg)
	}

	return res
}

func (fl *FileLayout) Present(cfg *PresentFileLayoutConfig) {
	if fl == nil {
		panic("Probably input yaml error, look for properly escaped strings and \" characters")
	}
	fl.inUTC = cfg.InUTC
	if fl.BaseName != "" {
		feng.Printf("# " + fl.BaseName + "\n")
	}

	presentStart := time.Now()
	for _, layout := range fl.Structs {
		feng.Printf(fl.presentStruct(layout, cfg))
	}

	fl.presentTime = time.Since(presentStart)

	fl.reportUnmappedByteCount()

	if cfg.ReportOverlapping {
		fl.reportOverlappingData()
	}

	if cfg.ReportUnmapped {
		fl.reportUnmappedData()
	}

	if fl.unseen {
		feng.Printf("\nUNSEEN data file. please submit a sample\n")
	}
}

func (fl *FileLayout) reportUnmappedByteCount() {

	mappedBytes := fl.MappedBytes()
	if mappedBytes < fl.size {
		unmapped := fl.size - mappedBytes
		unmappedPct := (float64(unmapped) / float64(fl.size)) * 100
		feng.Printf("0x%04x (%d) unmapped bytes (%.1f%%)\n", unmapped, unmapped, unmappedPct)
	} else if mappedBytes > fl.size {
		overflow := mappedBytes - fl.size

		feng.Printf("TOO MANY BYTES MAPPED! expected 0x%04x bytes but got 0x%04x. That is %d bytes too many!\n", fl.size, mappedBytes, overflow)
	} else {
		feng.Printf("EOF\n")
	}

	feng.Printf("\n---STAT---\n")
	feng.Printf("FILE SIZE : %d / 0x%06x / %s\n", fl.size, fl.size, ByteCountSI(fl.size))

	pctRead := (float64(fl.bytesRead) / float64(fl.size)) * 100

	feng.Printf("BYTES READ: %d (%.1f%%)\n", fl.bytesRead, pctRead) // XXX show bytes read in % of file size

	if fl.bytesImported > 0 {
		feng.Printf("BYTES IMPORTED: %d\n", fl.bytesImported)
	}

	if len(fl.previousOffsets) != 0 {
		feng.Printf("WARNING UNPOPPED OFFSETS: %#v (indicates buggy template)\n", fl.previousOffsets)
	}

	if fl.measureTime {
		feng.Printf("\n---TIME---\n")
		feng.Printf("EVALUATED EXPRESSIONS: %d (%v)\n", fl.evaluatedExpressions, fl.evaluatedExpressionTime)

		feng.Printf("MEASURE: template parsed in %v (other templates = %v)\n", fl.evaluationTime, fl.totalEvaluationTimeUntilMatch-fl.evaluationTime)
		feng.Printf("PRESENT TIME: %v\n", fl.presentTime)
	}

}

func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func (fl *FileLayout) reportOverlappingData() {
	feng.Printf("TODO: report overlapping bytes")
}

func (fl *FileLayout) reportUnmappedData() {
	unmappedRanges := []dataRange{}
	r := dataRange{offset: -1}
	log.Info().Msgf("reportUnmappedData start")
	for i := int64(0); i < int64(fl.size); i++ {
		if !fl.isMappedByte(i) {
			if r.offset == -1 {
				r.offset = i
				r.length = 1
			} else if i >= r.offset && i <= r.offset+r.length {
				r.length++
			} else {
				unmappedRanges = append(unmappedRanges, r)
				r = dataRange{offset: -1}
			}
		}
	}
	if r.offset != -1 {
		unmappedRanges = append(unmappedRanges, r)
	}
	log.Info().Msgf("reportUnmappedData unmappedRanges len %d", len(unmappedRanges))

	maxBytesShown := int64(32)
	for _, ur := range unmappedRanges {
		end := ur.length
		trail := ""
		if ur.length > maxBytesShown {
			end = maxBytesShown
			trail = " .."
		}
		lastOffset := ur.offset + ur.length - 1

		rawData, _ := fl.peekBytesMainFile(ur.offset, end)

		if lastOffset != ur.offset {
			feng.Printf("  [%06x-%06x] u8[%d] \t% 02x%s\n", ur.offset, lastOffset, ur.length, rawData, trail)
		} else {
			feng.Printf("  [%06x] u8 \t% 02x%s\n", ur.offset, rawData, trail)
		}
	}
}

type dataRange struct {
	offset int64
	length int64
}

// TODO FIXME: isMappedByte is extremely slow on a large file (300 mb) !!! it could take advantage of sizes of known structs and just skip ahead ???
func (fl *FileLayout) isMappedByte(offset int64) bool {
	for _, layout := range fl.Structs {
		for _, field := range layout.Fields {
			if offset >= field.Offset && offset < field.Offset+field.Length {
				return true
			}
		}
		for _, child := range layout.Children {
			for _, field := range child.Fields {
				if offset >= field.Offset && offset < field.Offset+field.Length {
					return true
				}
			}
		}
	}
	return false
}

// return the number of mapped bytes
func (fl *FileLayout) MappedBytes() int64 {
	count := int64(0)
	for _, layout := range fl.Structs {
		for _, field := range layout.Fields {
			if field.ImportFile == "" {
				count += field.Length
			}
		}
		for _, child := range layout.Children {
			for _, field := range child.Fields {
				if field.ImportFile == "" {
					count += field.Length
				}
			}
		}
	}
	return count
}

func (fl *FileLayout) GetStruct(name string) (*Struct, error) {
	for _, str := range fl.Structs {
		if str.Name == name {
			return str, nil
		}
	}
	return nil, fmt.Errorf("GetStruct: %s not found", name)
}

// finds the first field named `structName`.`fieldName`
func (fl *FileLayout) GetInt(s string, df *value.DataField) (int64, error) {
	log.Debug().Msgf("GetInt: searching for '%s'", s)

	n, err := fl.EvaluateExpression(s, df)
	if err != nil {
		// XXX this is critical error and template must be fixed
		log.Fatal().Err(err).Msg("GetInt FAILURE on '" + s + "'")
	}
	log.Debug().Msgf("GetInt: %s => %d", s, n)
	return n, err
}

func (fl *FileLayout) isPatternVariableName(s string, df *value.DataField) bool {
	if df != nil {
		s = strings.ReplaceAll(s, "self.", df.Label+".")
	}
	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		return false
	}
	structName := parts[0]
	fieldName := parts[1]

	str, err := fl.GetStruct(structName)
	if err != nil {
		log.Print(err)
		return false
	}
	for _, field := range str.Fields {
		if field.Format.Label == fieldName {
			return true
		}
	}
	return false
}

// returns the pattern matched value of field named `structName`.`fieldName`
func (fl *FileLayout) MatchedValue(s string, df *value.DataField) (string, error) {

	if df != nil {
		s = strings.ReplaceAll(s, "self.", df.Label+".")
	}

	log.Trace().Msgf("MatchedValue: searching for '%s'", s)

	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		log.Debug().Msgf("MatchedValue: unexpected format '%s'", s)
		return s, nil
	}
	structName := parts[0]
	fieldName := parts[1]

	str, err := fl.GetStruct(structName)
	if err != nil {
		return "", err
	}

	for _, field := range str.Fields {
		if field.Format.Label == fieldName {
			if len(field.MatchedPatterns) == 0 {
				b, err := fl.peekBytes(&field)
				if err != nil {
					panic(err)
				}

				return fl.PresentFieldValue(&field, b), nil
			}
			for _, child := range field.MatchedPatterns {
				log.Trace().Msgf("MatchedValue: %s => %s", fieldName, child.Label)
				return child.Label, nil
			}
		}
	}
	log.Trace().Msgf("MatchedValue: '%s' not found", s)
	return s, nil
}

// finds the first field named `structName`.`fieldName`
// returns: kind, bytes, error
func (fl *FileLayout) GetValue(s string, df *value.DataField) (string, []byte, error) {
	if df != nil {
		s = strings.ReplaceAll(s, "self.", df.Label+".")
	}

	log.Trace().Msgf("GetValue: searching for '%s'", s)

	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		return "", nil, fmt.Errorf("GetValue: unexpected format '%s'", s)
	}
	structName := parts[0]
	fieldName := parts[1]
	childName := ""
	if len(parts) > 2 {
		childName = parts[2]
	}

	str, err := fl.GetStruct(structName)
	if err != nil {
		return "", nil, err
	}

	for _, field := range str.Fields {
		log.Trace().Msgf("GetValue: want %s, got %s", fieldName, field.Format.Label)

		if field.Format.Label == fieldName {
			switch childName {
			case "offset":
				val := value.U64toBytesBigEndian(uint64(field.Offset), 8)
				return "u64", val, nil
			case "len":
				val := value.U64toBytesBigEndian(uint64(field.Length), 8)
				return "u64", val, nil
			}

			if !field.Format.IsSimpleUnit() || childName == "" {
				data, err := fl.peekBytes(&field)
				if err != nil {
					return "", nil, err
				}
				return field.Format.Kind, data, nil
			}

			for _, child := range field.MatchedPatterns {
				if child.Label == childName {
					return field.Format.Kind, child.Value, nil
				}
			}
		}
	}

	return "", nil, fmt.Errorf("GetValue: '%s' not found", s)
}

// finds the first field named `structName`.`fieldName`
// returns: offset,error
func (fl *FileLayout) GetOffset(query string, df *value.DataField) (int, error) {

	if df != nil {
		query = strings.ReplaceAll(query, "self.", df.Label+".")
	}

	log.Trace().Msgf("GetOffset: searching for '%s'", query)

	parts := strings.SplitN(query, ".", 3)
	if len(parts) < 2 {
		return 0, fmt.Errorf("GetOffset: unexpected format '%s'", query)
	}
	structName := parts[0]
	fieldName := parts[1]

	str, err := fl.GetStruct(structName)
	if err != nil {
		return 0, err
	}

	for _, field := range str.Fields {
		log.Trace().Msgf("GetOffset: want %s, got %s", fieldName, field.Format.Label)

		if field.Format.Label == fieldName {
			return int(field.Offset), nil
		}
	}

	return 0, fmt.Errorf("GetOffset: '%s' not found", query)
}

// finds the first field named `structName`.`fieldName`
// returns: length,error
func (fl *FileLayout) GetLength(s string, df *value.DataField) (int, error) {

	if df != nil {
		s = strings.ReplaceAll(s, "self.", df.Label+".")
	}

	log.Trace().Msgf("GetLength: searching for '%s'", s)

	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		return 0, fmt.Errorf("GetLength: unexpected format '%s'", s)
	}
	structName := parts[0]
	fieldName := parts[1]

	str, err := fl.GetStruct(structName)
	if err != nil {
		return 0, err
	}

	for _, field := range str.Fields {
		log.Trace().Msgf("GetLength: want %s, got %s", fieldName, field.Format.Label)

		if field.Format.Label == fieldName {
			log.Debug().Msgf("Read length %d from %s.%s", int(field.Length), structName, fieldName)
			return int(field.Length), nil
		}
	}

	return 0, fmt.Errorf("GetLength: '%s' not found", s)
}

// returns unitLength, totalLength (in bytes)
func (fl *FileLayout) GetAddressLengthPair(df *value.DataField) (int64, int64) {

	unitLength := df.SingleUnitSize()
	rangeLength := int64(1)
	var err error

	if df.Range != "" {
		if df.RangeVal == 0 {
			log.Debug().Msgf("Calculating initial value for df.Range: %s", df.Range)

			//val, err := fl.evaluateExpressionWithExistingVariables(df.Range, df)
			val, err := fl.EvaluateExpression(df.Range, df)
			if err != nil {
				panic(err)
			}
			df.RangeVal = int64(val)
		}
		rangeLength = int64(df.RangeVal)

		if err != nil {
			log.Fatal().Err(err).Msgf("failed")
		}
	}
	totalLength := unitLength * int64(rangeLength)
	log.Trace().Msgf("GetAddressLengthPair: unitLength %d * rangeLength %d = totalLength %d", unitLength, rangeLength, totalLength)

	return unitLength, totalLength
}

// presents the underlying type as it is known in the template format
func (fl *FileLayout) PresentType(df *value.DataField) string {
	if df.Slice {
		return fmt.Sprintf("%s[]", df.Kind)
	}

	if df.Range != "" {
		unitLength, totalLength := fl.GetAddressLengthPair(df)
		fieldLength := totalLength / unitLength
		return fmt.Sprintf("%s[%d]", df.Kind, fieldLength)
	}
	return df.Kind
}

// replace variables with their values
func (fl *FileLayout) ExpandVariables(s string, df *value.DataField) (string, error) {

	log.Trace().Msgf("ExpandVariables: %s", s)

	for {
		expanded, err := fl.expandVariable(s, df)
		if err != nil {
			return "", err
		}
		if expanded == s {
			break
		}
		log.Trace().Msgf("ExpandVariables: %s => %s", s, expanded)

		s = expanded
	}

	return s, nil
}

func (fl *FileLayout) expandVariable(s string, df *value.DataField) (string, error) {
	log.Trace().Msgf("expandVariable: %s", s)

	matches := variableExpressionRE.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		log.Trace().Msgf("expandVariable: NO MATCH")
		return s, nil
	}

	indexes := variableExpressionRE.FindAllStringSubmatchIndex(s, -1)

	for idx, match := range matches {

		key := strings.TrimSpace(match[0])
		if key == "" || isIntegerString(key) {
			continue
		}

		//key = strings.ReplaceAll(key, "self.", df.Label+".")
		kind, val, err := fl.GetValue(key, df)
		if err != nil {
			s, err := fl.EvaluateExpression(key, df)

			log.Trace().Msgf("expandVariable: evaluated expression '%s' to %s == %d", key, kind, s)

			return fmt.Sprintf("%d", s), err
		}

		log.Trace().Msgf("expandVariable: MATCHED %s to %s %v", key, kind, val)

		i := value.AsUint64(kind, val)

		return s[0:indexes[idx][0]] + fmt.Sprintf("%d", i) + s[indexes[idx][1]:], nil
	}

	return s, nil
}

// returns true if string represents an integer
func isIntegerString(s string) bool {
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	return false
}
