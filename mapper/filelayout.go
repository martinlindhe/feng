package mapper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

// parsed file data from a template "layout"
type FileLayout struct {
	Structs []Struct

	// pointer to its internal yaml representation, so we can access "constants"
	DS *template.DataStructure

	// current endian ("big", "little")
	endian string

	// current output data filename
	filename string

	// current offset
	offset uint64

	// previous offset (restore it with "offset: restore")
	previousOffsets []uint64

	// counts how many times the offset was changed in order to stop recursion
	offsetChanges uint64

	// total size of data (FILE_SIZE)
	size uint64

	// default extension
	Extension string

	// lastpath/filename-without-ext, eg "archives/zip"
	BaseName string

	// the file that was processed to produce this FileLayout struct
	DataFileName string

	// the raw data underlying the structure. used for peek()
	rawData []byte

	// if unseen, ask user to submit a sample
	unseen bool

	// present datetimes in UTC
	inUTC bool
}

// pop last offset from previousOffsets list
func (fl *FileLayout) popLastOffset() (v uint64) {
	if len(fl.previousOffsets) == 0 {
		panic("cannot pop offset, no offsets have been pushed")
	}

	v, fl.previousOffsets = fl.previousOffsets[len(fl.previousOffsets)-1], fl.previousOffsets[:len(fl.previousOffsets)-1]
	return
}

// push current offset to previousOffsets list
func (fl *FileLayout) pushOffset() uint64 {
	fl.previousOffsets = append(fl.previousOffsets, fl.offset)
	return fl.offset
}

// parsed file data section from a template "struct"
type Struct struct {
	// unique name of this instance of the struct
	Name string

	// child nodes
	Children []Struct

	// additional decoration
	Label string

	Fields []Field

	// slice-based counter index, 0-based
	Index int

	// when evaluated once, struct can be skipped by eval function
	evaluated bool
}

type Field struct {
	Offset uint64
	Length uint64

	// value in network byte order (big)
	Value []byte

	// on-disk endianness
	Endian string

	// underlying data structure
	Format value.DataField

	// matched patterns
	MatchedPatterns []value.MatchedPattern

	// filename for the next output data
	Filename string
}

func (field *Field) Present() string {
	return field.Format.Present(field.Value, field.Endian)
}

var (
	variableExpressionRE = regexp.MustCompile(`([\w .+\-*/()<>"&]+)`)
)

const (
	maxHexDisplayLength = 0x20
)

func shortFloat(f float32) string {
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

// returns the value of the data type (field.Format.Kind)
func (fl *FileLayout) GetFieldValue(field *Field) interface{} {
	b := field.Value
	switch field.Format.Kind {
	case "compressed:deflate", "compressed:lz4", "compressed:zlib", "raw:u8":
		return ""

	case "f32":
		if field.Format.Slice || field.Format.Range != "" {
			return ""
		}
		return shortFloat(math.Float32frombits(uint32(value.AsUint64Raw(b))))

	case "xyzm32":
		if field.Format.Slice || field.Format.Range != "" {
			return ""
		}
		return fmt.Sprintf("%s, %s, %s, %s",
			shortFloat(math.Float32frombits(uint32(value.AsUint64Raw(b[:4])))),
			shortFloat(math.Float32frombits(uint32(value.AsUint64Raw(b[4:8])))),
			shortFloat(math.Float32frombits(uint32(value.AsUint64Raw(b[8:12])))),
			shortFloat(math.Float32frombits(uint32(value.AsUint64Raw(b[12:16])))),
		)

	case "u8", "u16", "u32", "u64":
		if field.Format.Slice && field.Format.Range == "" {
			panic("FIXME present slice " + field.Format.Kind)
		}
		if !field.Format.Slice && field.Format.Range != "" {
			unitLength, totalLength := fl.GetAddressLengthPair(&field.Format)
			values := []interface{}{}
			switch field.Format.Kind {
			case "u8":
				return field.Value
			case "u16":
				for i := uint64(0); i < totalLength; i += unitLength {
					if field.Endian == "big" {
						values = append(values, uint64(binary.BigEndian.Uint16(b[i:])))
					} else {
						values = append(values, uint64(binary.LittleEndian.Uint16(b[i:])))
					}
				}
			case "u32":
				for i := uint64(0); i < totalLength; i += unitLength {
					if field.Endian == "big" {
						values = append(values, uint64(binary.BigEndian.Uint32(b[i:])))
					} else {
						values = append(values, uint64(binary.LittleEndian.Uint32(b[i:])))
					}
				}
			default:
				panic("handle " + field.Format.Kind)
			}
			return values
		}
		return int(value.AsUint64Raw(b))

	case "i8", "i16", "i32", "i64":
		if field.Format.Slice || field.Format.Range != "" {
			return ""
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
		}

	case "ascii":
		v, _ := value.AsciiPrintableString(b, len(b))
		return v

	case "asciiz":
		v, _ := value.AsciiZString(b, len(b))
		return v

	case "utf16":
		return value.Utf16String(b)

	case "utf16z":
		return value.Utf16zString(b)

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
		return fmt.Sprintf("(%d, %d, %d)", b[0], b[1], b[2])

	case "vu32":
		got, _, _, _ := value.ReadVariableLengthU32(bytes.NewReader(b))
		return got

	case "vu64":
		got, _, _, _ := value.ReadVariableLengthU64(bytes.NewReader(b))
		return got
	}

	log.Fatal().Msgf("don't know how to present %s (slice:%v, range:%s): %v", field.Format.Kind, field.Format.Slice, field.Format.Range, b)
	return ""
}

// presents the value of the data type (field.Format.Kind) in a human-readable form
func (fl *FileLayout) PresentFieldValue(field *Field) string {
	// XXX a lot of stuff is re-evaluated here, should reuse data from parsing
	b := field.Value
	switch field.Format.Kind {
	case "compressed:deflate", "compressed:lz4", "compressed:zlib", "raw:u8":
		return ""

	case "f32":
		if field.Format.Slice && field.Format.Range == "" {
			panic("FIXME present slice")
		}

		if !field.Format.Slice && field.Format.Range != "" {
			values := []string{}
			val := 0
			skipRest := false

			unitLength, totalLength := fl.GetAddressLengthPair(&field.Format)

			for i := uint64(0); i < totalLength; i += unitLength {
				val++
				if field.Endian == "big" {
					values = append(values, shortFloat(math.Float32frombits(binary.BigEndian.Uint32(b[i:]))))
				} else {
					values = append(values, shortFloat(math.Float32frombits(binary.LittleEndian.Uint32(b[i:]))))
				}
				if val >= 3 {
					skipRest = true
					break
				}
			}
			if skipRest {
				return "[" + strings.Join(values, ", ") + " ... ]"
			}
			return "[" + strings.Join(values, ", ") + "]"
		}
		return shortFloat(math.Float32frombits(uint32(value.AsUint64Raw(b))))

	case "u8", "u16", "u32", "u64":
		if field.Format.Slice && field.Format.Range == "" {
			panic("FIXME present slice")
		}
		if !field.Format.Slice && field.Format.Range != "" {
			unitLength, totalLength := fl.GetAddressLengthPair(&field.Format)

			values := []string{}
			val := 0
			skipRest := false

			switch field.Format.Kind {
			case "u8":
				return ""
			case "u16":
				for i := uint64(0); i < totalLength; i += unitLength {
					val++
					if field.Endian == "big" {
						values = append(values, fmt.Sprintf("%d", binary.BigEndian.Uint16(b[i:])))
					} else {
						values = append(values, fmt.Sprintf("%d", binary.LittleEndian.Uint16(b[i:])))
					}
					if val >= 3 {
						skipRest = true
						break
					}
				}
			case "u32":
				for i := uint64(0); i < totalLength; i += unitLength {
					val++
					if field.Endian == "big" {
						values = append(values, fmt.Sprintf("%d", binary.BigEndian.Uint32(b[i:])))
					} else {
						values = append(values, fmt.Sprintf("%d", binary.LittleEndian.Uint32(b[i:])))
					}
					if val >= 3 {
						skipRest = true
						break
					}
				}
			default:
				panic("FIXME handle " + field.Format.Kind)
			}

			if skipRest {
				return "[" + strings.Join(values, ", ") + " ... ]"
			}
			return "[" + strings.Join(values, ", ") + "]"
		}
		return fmt.Sprintf("%d", value.AsUint64Raw(b))

	case "i8", "i16", "i32", "i64",
		"ascii", "asciiz", "xyzm32", "utf16", "utf16z", "time_t_32", "filetime", "dostime", "dosdate", "dostimedate",
		"rgb8", "vu32", "vu64":
		res := fl.GetFieldValue(field).(string)
		return presentStringValue(res)
	}

	log.Fatal().Msgf("don't know how to present %s (slice:%v, range:%s): %v", field.Format.Kind, field.Format.Slice, field.Format.Range, b)
	return ""
}

// renders lines of ascii to present the data field for humans
func (fl *FileLayout) presentField(field *Field, showRaw bool) string {
	kind := fl.PresentType(&field.Format)
	if (field.Format.Kind != "vu32" && field.Format.Kind != "vu64") && field.Format.SingleUnitSize() > 1 {
		// XXX hacky way of skipping variable length fields
		if field.Endian == "little" {
			kind += " le"
		} else {
			kind += " be"
		}
	}

	fieldValue := strings.TrimRight(fl.PresentFieldValue(field), " ")

	res := ""
	if !showRaw {
		res = fmt.Sprintf("  [%06x] %-30s %-16s %-30s",
			field.Offset, field.Format.Label, kind, fieldValue)
	} else {
		hexValue := ""
		if len(field.Value) <= maxHexDisplayLength {
			hexValue = fmt.Sprintf("% 02x", field.Value)
		} else {
			hexValue = fmt.Sprintf("% 02x ...", field.Value[0:maxHexDisplayLength])
		}
		res = fmt.Sprintf("  [%06x] %-30s %-16s %-30s %-20s",
			field.Offset, field.Format.Label, kind, fieldValue, hexValue)
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

func (fl *FileLayout) PresentStructureTree(structs []Struct) string {
	res := fmt.Sprintf("# structure tree of %s\n", fl.DataFileName)
	for _, layout := range structs {
		res += fl.presentStructureTreeNode(layout, 0)
	}
	return res
}
func (fl *FileLayout) presentStructureTreeNode(layout Struct, indent int) string {
	prefix := strings.Repeat(" ", indent)
	res := ""
	heading := prefix + layout.Name
	if layout.Label != "" {
		heading += ` "` + layout.Label + `"`
	}
	if len(layout.Fields) == 0 {
		res += "   empty struct"
	}
	res += heading + "\n"
	for _, child := range layout.Children {
		res += prefix + fl.presentStructureTreeNode(child, indent+2)
	}
	return res
}

type PresentFileLayoutConfig struct {
	ShowRaw           bool
	ReportUnmapped    bool
	ReportOverlapping bool
	InUTC             bool
}

func (fl *FileLayout) presentStruct(layout *Struct, showRaw bool) string {
	if len(layout.Fields) == 0 {
		if DEBUG {
			feng.Yellow("skip empty struct '%s'\n", layout.Name)
		}
		return ""
	}
	heading := layout.Name
	if layout.Label != "" {
		heading += " \"" + layout.Label + "\""
	}
	res := heading + "\n"
	for _, field := range layout.Fields {
		res += fl.presentField(&field, showRaw)
	}
	res += "\n"

	for _, child := range layout.Children {
		res += fl.presentStruct(&child, showRaw)
	}

	return res
}

func (fl *FileLayout) Present(cfg *PresentFileLayoutConfig) (res string) {
	if fl == nil {
		panic("Probably input yaml error, look for properly escaped strings and \" characters")
	}
	fl.inUTC = cfg.InUTC
	res = "# " + fl.BaseName + "\n"
	for _, layout := range fl.Structs {
		res += fl.presentStruct(&layout, cfg.ShowRaw)
	}

	res += fl.reportUnmappedByteCount()

	if cfg.ReportOverlapping {
		res += fl.reportOverlappingData()
	}

	if cfg.ReportUnmapped {
		res += fl.reportUnmappedData()
	}

	if fl.unseen {
		res += "\nUNSEEN data file. please submit a sample\n"
	}

	return
}

func (fl *FileLayout) reportUnmappedByteCount() string {
	res := ""
	mappedBytes := fl.MappedBytes()
	if mappedBytes < fl.size {
		unmapped := fl.size - mappedBytes
		unmappedPct := (float64(unmapped) / float64(fl.size)) * 100
		res += fmt.Sprintf("0x%04x (%d) unmapped bytes (%.1f%%)\n", unmapped, unmapped, unmappedPct)
	} else if mappedBytes > fl.size {
		overflow := mappedBytes - fl.size
		res += fmt.Sprintf("TOO MANY BYTES MAPPED! expected 0x%04x bytes but got 0x%04x. That is %d bytes too many!\n", fl.size, mappedBytes, overflow)
	} else {
		res += "EOF\n"
	}
	return res
}

func (fl *FileLayout) reportOverlappingData() string {
	// XXX report overlapping bytes.

	return ""
}

func (fl *FileLayout) reportUnmappedData() string {
	res := ""
	unmappedRanges := []dataRange{}
	r := dataRange{offset: -1}
	for i := 0; i < int(fl.size); i++ {
		if !fl.isMappedByte(uint64(i)) {
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

	maxBytesShown := 32
	for _, ur := range unmappedRanges {
		end := ur.offset + ur.length
		trail := ""
		if ur.length > maxBytesShown {
			end = ur.offset + maxBytesShown
			trail = " .."
		}
		lastOffset := ur.offset + ur.length - 1
		if lastOffset != ur.offset {
			res += fmt.Sprintf("  [%06x-%06x] u8[%d] \t% 02x%s\n", ur.offset, lastOffset, ur.length, fl.rawData[ur.offset:end], trail)
		} else {
			res += fmt.Sprintf("  [%06x] u8 \t% 02x%s\n", ur.offset, fl.rawData[ur.offset:end], trail)
		}
	}
	return res
}

type dataRange struct {
	offset int
	length int
}

func (fl *FileLayout) isMappedByte(offset uint64) bool {
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
func (fl *FileLayout) MappedBytes() uint64 {
	count := uint64(0)
	for _, layout := range fl.Structs {
		for _, field := range layout.Fields {
			count += field.Length
		}
		for _, child := range layout.Children {
			for _, field := range child.Fields {
				count += field.Length
			}
		}
	}
	return count
}

func (fl *FileLayout) GetStruct(name string) (*Struct, error) {
	for _, str := range fl.Structs {
		if str.Name == name {
			return &str, nil
		}
	}
	return nil, fmt.Errorf("GetStruct: %s not found", name)
}

// finds the first field named `structName`.`fieldName`
func (fl *FileLayout) GetInt(s string, df *value.DataField) (uint64, error) {
	if DEBUG {
		log.Printf("GetInt: searching for '%s'", s)
	}

	n, err := fl.EvaluateExpression(s, df)
	if err != nil {
		// XXX this is critical error and template must be fixed
		log.Fatal().Err(err).Msg("GetInt FAILURE on '" + s + "'")
	}
	if DEBUG {
		log.Printf("GetInt: %s => %d", s, n)
	}
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
				return fl.PresentFieldValue(&field), nil
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
				val := value.U64toBytesBigEndian(field.Offset, 8)
				return "u64", val, nil
			case "len":
				val := value.U64toBytesBigEndian(field.Length, 8)
				return "u64", val, nil
			}

			if !field.Format.IsSimpleUnit() || childName == "" {
				return field.Format.Kind, field.Value, nil
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
			return int(field.Length), nil
		}
	}

	return 0, fmt.Errorf("GetLength: '%s' not found", s)
}

// returns unitLength, totalLength
func (fl *FileLayout) GetAddressLengthPair(df *value.DataField) (uint64, uint64) {

	unitLength := df.SingleUnitSize()
	rangeLength := uint64(1)
	var err error

	if df.Range != "" {
		if df.RangeVal == 0 {
			// XXX permanently store and reuse the calculated range length. faster lookup & avoid bug with changing offset value... ?!
			// XXX FIXME TODO! !!! REFACTOR THIS:
			// STORES CACHED RESULT OF CALCULATION
			log.Debug().Msgf("Calculating initial value for df.Range: %s", df.Range)
			val, err := fl.EvaluateExpression(df.Range, df)
			if err != nil {
				panic(err)
			}
			df.RangeVal = int64(val)
		}
		rangeLength = uint64(df.RangeVal)

		if err != nil {
			log.Fatal().Err(err)
		}
	}
	totalLength := unitLength * uint64(rangeLength)
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
