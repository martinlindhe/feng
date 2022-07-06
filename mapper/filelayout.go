package mapper

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

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

	// current offset
	offset uint64

	// previous offset (restore it with "offset: restore")
	previousOffset uint64

	// counts how many times the offset was changed in order to stop recursion
	offsetChanges uint64

	// total size of data (FILE_SIZE)
	size uint64

	// default extension
	Extension string

	// lastpath/filename-without-ext, eg "archives/zip"
	BaseName string
}

// parsed file data section from a template "struct"
type Struct struct {
	Label      string
	decoration string

	Fields []Field

	// slice-based counter index, 0-based
	Index int
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
}

func (field *Field) Present() string {
	return field.Format.Present(field.Value)
}

var (
	variableExpressionRE = regexp.MustCompile(`([\w .+\-*/()<>"&]+)`)
)

const (
	maxHexDisplayLength = 0x20
)

// renders lines of ascii to present the data field for humans
func (fl *FileLayout) presentField(field *Field, hideRaw bool) string {
	kind := fl.PresentType(&field.Format)
	if field.Format.SingleUnitSize() > 1 {
		if field.Endian == "little" {
			kind += " le"
		} else {
			kind += " be"
		}
	}

	fieldValue := field.Present()

	res := ""
	if hideRaw {
		res = fmt.Sprintf("  [%06x] %-30s %-16s %-21s\n",
			field.Offset, field.Format.Label, kind, fieldValue)
	} else {
		hexValue := ""
		if len(field.Value) <= maxHexDisplayLength {
			hexValue = fmt.Sprintf("% 02x", field.Value)
		} else {
			hexValue = fmt.Sprintf("% 02x ...", field.Value[0:maxHexDisplayLength])
		}
		res = fmt.Sprintf("  [%06x] %-30s %-16s %-21s %-20s",
			field.Offset, field.Format.Label, kind, fieldValue, hexValue)
		res = strings.TrimRight(res, " ") + "\n"
	}

	for _, child := range field.MatchedPatterns {
		pretty := fmt.Sprintf("%d", child.Value)
		if child.Operation == "eq" && child.Parsed != "" {
			pretty = child.Parsed
		}

		raw := ""
		switch child.Ones {
		case 0, 1, 2, 3:
		case 4, 5, 6, 7, 8:
			raw = fmt.Sprintf("%02x", child.Value)
		case 13:
			raw = fmt.Sprintf("%04x", child.Value)
		case 24:
			raw = fmt.Sprintf("%06x", child.Value)
		case 31, 32:
			raw = fmt.Sprintf("%08x", child.Value)
		default:
			panic(fmt.Sprintf("handle bit count %d", child.Ones))
		}

		// add a space between each hex byte
		rawParts := []string{}
		for i := 0; i < len(raw); i += 2 {
			rawParts = append(rawParts, raw[i:i+2])
		}
		raw = strings.Join(rawParts, " ")

		line := fmt.Sprintf("           - %-28s %-16s %-21s %s", child.Label, child.Operation, pretty, raw)
		res += strings.TrimRight(line, " ") + "\n"
	}
	return res
}

func (fl *FileLayout) Present(hideRaw bool) (res string) {
	res = "# " + fl.BaseName + "\n"
	for _, layout := range fl.Structs {
		if len(layout.Fields) == 0 {
			if DEBUG {
				feng.Yellow("skip empty struct '%s'\n", layout.Label)
			}
			continue
		}
		heading := layout.Label
		if layout.decoration != "" {
			heading += " " + layout.decoration
		}
		res += heading + "\n"
		for _, field := range layout.Fields {
			res += fl.presentField(&field, hideRaw)
		}
		res += "\n"
	}

	mappedBytes := fl.MappedBytes()
	if mappedBytes < fl.size {
		unmapped := fl.size - mappedBytes
		res += fmt.Sprintf("0x%04x (%d) unmapped bytes\n", unmapped, unmapped)
	} else if mappedBytes > fl.size {
		res += fmt.Sprintf("TOO MANY BYTES MAPPED! expected 0x%04x bytes but got 0x%04x\n", fl.size, mappedBytes)
	} else {
		res += "EOF\n"
	}
	return
}

// return the number of mapped bytes
func (fl *FileLayout) MappedBytes() uint64 {
	count := uint64(0)
	for _, layout := range fl.Structs {
		for _, field := range layout.Fields {
			count += field.Length
		}
	}
	return count
}

func (fl *FileLayout) GetStruct(name string) (*Struct, error) {
	for _, str := range fl.Structs {
		if DEBUG {
			//log.Printf("GetStruct: want %s, got %s", name, str.Label)
		}
		if str.Label == name {
			return &str, nil
		}
	}
	return nil, fmt.Errorf("GetStruct: %s not found", name)
}

// finds the first field named `structName`.`fieldName`
func (fl *FileLayout) GetInt(s string, df *value.DataField) (uint64, error) {

	if df != nil {
		s = strings.ReplaceAll(s, "self.offset", fmt.Sprintf("%d", fl.offset))

		s = strings.ReplaceAll(s, "self.", df.Label+".")
	}

	if DEBUG {
		log.Printf("GetInt: searching for '%s'", s)
	}

	n, err := fl.EvaluateExpression(s)
	if err != nil {
		// XXX this is critical error and template must be fixed
		log.Fatal("GetInt FAILURE:", err)
	}
	if DEBUG {
		log.Printf("GetInt: %s => %d", s, n)
	}
	return n, err
}

// returns the pattern matched value of field named `structName`.`fieldName`
func (fl *FileLayout) MatchedValue(s string, df *value.DataField) (string, error) {

	if df != nil {
		s = strings.ReplaceAll(s, "self.", df.Label+".")
	}

	if DEBUG {
		log.Printf("MatchedValue: searching for '%s'", s)
	}

	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		//feng.Red("MatchedValue: unexpected format '%s'", s)
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
				return field.Present(), nil
			}
			for _, child := range field.MatchedPatterns {
				if DEBUG {
					log.Printf("MatchedValue: %s => %s", fieldName, child.Label)
				}
				return child.Label, nil
			}
		}
	}
	log.Printf("MatchedValue: '%s' not found", s)
	return s, nil
}

// finds the first field named `structName`.`fieldName`
// returns: kind, bytes, error
func (fl *FileLayout) GetValue(s string, df *value.DataField) (string, []byte, error) {
	if df != nil {
		s = strings.ReplaceAll(s, "self.", df.Label+".")
	}

	if DEBUG {
		log.Printf("GetValue: searching for '%s'", s)
	}

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
		if DEBUG {
			log.Printf("GetValue: want %s, got %s", fieldName, field.Format.Label)
		}
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
					val := value.U64toBytesBigEndian(child.Value, field.Format.SingleUnitSize())
					if DEBUG {
						log.Printf("-- matched pattern %s = %d", child.Label, val)
					}
					return field.Format.Kind, val, nil
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

	if DEBUG {
		log.Printf("GetOffset: searching for '%s'", query)
	}

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
		if DEBUG {
			log.Printf("GetOffset: want %s, got %s", fieldName, field.Format.Label)
		}
		if field.Format.Label == fieldName {
			return int(field.Offset), nil
		}
	}

	return 0, fmt.Errorf("GetOffset: '%s' not found", query)
}

// finds the first field named `structName`.`fieldName`
// returns: offset,error
func (fl *FileLayout) GetLength(s string, df *value.DataField) (int, error) {

	if df != nil {
		s = strings.ReplaceAll(s, "self.", df.Label+".")
	}

	if DEBUG {
		log.Printf("GetLength: searching for '%s'", s)
	}

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
		if DEBUG {
			log.Printf("GetLength: want %s, got %s", fieldName, field.Format.Label)
		}
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
			val, err := fl.EvaluateExpression(df.Range)
			if err != nil {
				panic(err)
			}
			df.RangeVal = int64(val)
		}
		rangeLength = uint64(df.RangeVal)

		if err != nil {
			log.Fatal(err)
		}
	}
	totalLength := unitLength * uint64(rangeLength)
	if DEBUG {
		log.Printf("GetAddressLengthPair: unitLength %d * rangeLength %d = totalLength %d", unitLength, rangeLength, totalLength)
	}
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

	s = strings.Replace(s, "self.offset", fmt.Sprintf("%d", fl.offset), 1)

	if DEBUG {
		log.Printf("ExpandVariables: %s", s)
	}

	for {
		expanded, err := fl.expandVariable(s, df)
		if err != nil {
			return "", err
		}
		if expanded == s {
			break
		}
		if DEBUG {
			log.Printf("ExpandVariables: %s => %s", s, expanded)
		}
		s = expanded
	}

	return s, nil
}

func (fl *FileLayout) expandVariable(s string, df *value.DataField) (string, error) {
	if DEBUG {
		log.Printf("expandVariable: %s", s)
	}
	matches := variableExpressionRE.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		if DEBUG {
			log.Printf("expandVariable: NO MATCH")
		}
		return s, nil
	}

	indexes := variableExpressionRE.FindAllStringSubmatchIndex(s, -1)

	for idx, match := range matches {

		key := strings.TrimSpace(match[0])
		if key == "" || isIntegerString(key) {
			continue
		}

		key = strings.ReplaceAll(key, "self.", df.Label+".")
		kind, val, err := fl.GetValue(key, df)
		if err != nil {
			s, err := fl.EvaluateExpression(key)

			if DEBUG {
				log.Printf("expandVariable: evaluated expression '%s' to %s == %d", key, kind, s)
			}

			return fmt.Sprintf("%d", s), err
		}

		if DEBUG {
			log.Printf("expandVariable: MATCHED %s to %s %v", key, kind, val)
		}

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
