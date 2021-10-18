package mapper

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/martinlindhe/feng/value"
)

// parsed file data from a template "layout"
type FileLayout struct {
	Structs []Struct

	// current endian ("big", "little")
	endian string

	// current offset
	offset uint64

	// total size of data (FILE_SIZE)
	size uint64

	// default extension
	Extension string
}

// parsed file data section from a template "struct"
type Struct struct {
	Label string

	Fields []Field
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

var (
	variableExpressionRE      = regexp.MustCompile(`([\w .+\-*/()<>]+)`)
	absoluteRangeExpressionRE = regexp.MustCompile(`([\d\s\+\-\*\/]+):([\d\s\+\-\*\/]+)`)

	red = color.New(color.FgRed).SprintfFunc()
)

const (
	maxHexDisplayLength = 0x20
)

// decodes simple value types for presentation
func (fl *FileLayout) PresentField(field *Field, hideRaw bool) string {
	kind := fl.PresentType(&field.Format)
	if field.Format.SingleUnitSize() > 1 {
		if field.Endian == "little" {
			kind += " le"
		} else {
			kind += " be"
		}
	}

	fieldValue := value.Present(field.Format, field.Value)

	res := ""
	if hideRaw {
		res = fmt.Sprintf("  [%06x] %-30s %-13s %-21s\n",
			field.Offset, field.Format.Label, kind, fieldValue)
	} else {
		hexValue := ""
		if len(field.Value) <= maxHexDisplayLength {
			hexValue = fmt.Sprintf("% 02x", field.Value)
		} else {
			hexValue = fmt.Sprintf("% 02x ...", field.Value[0:maxHexDisplayLength])
		}
		res = fmt.Sprintf("  [%06x] %-30s %-13s %-21s %-20s\n",
			field.Offset, field.Format.Label, kind, fieldValue, hexValue)
	}

	for _, child := range field.MatchedPatterns {
		res += fmt.Sprintf("           - %-28s %-13s %d\n", child.Label, child.Operation, child.Value)
	}
	return res
}

func (fl *FileLayout) Present(hideRaw bool) {
	for _, layout := range fl.Structs {
		fmt.Printf("%s\n", layout.Label)
		for _, field := range layout.Fields {
			fmt.Print(fl.PresentField(&field, hideRaw))
		}
		fmt.Println()
	}

	mappedBytes := fl.MappedBytes()
	if mappedBytes < fl.size {
		unmapped := fl.size - mappedBytes
		fmt.Println(red("0x%04x (%d) unmapped bytes", unmapped, unmapped))
	} else if mappedBytes > fl.size {
		fmt.Println(red("TOO MANY BYTES MAPPED! expected 0x%04x bytes but got 0x%04x", fl.size, mappedBytes))
	} else {
		fmt.Println("EOF")
	}
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
	if DEBUG {
		log.Printf("GetStruct: searching for %s", name)
	}
	for _, str := range fl.Structs {
		if str.Label == name {
			return &str, nil
		}
	}
	return nil, fmt.Errorf("GetStruct: %s not found", name)
}

// finds the first field named `structName`.`fieldName`
// returns: kind, bytes, error
func (fl *FileLayout) GetValue(s string, df *value.DataField) (string, []byte, error) {

	s = strings.Replace(s, "self.", df.Label+".", 1)

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

// returns unitLength, totalLength
func (fl *FileLayout) GetLength(df *value.DataField) (uint64, uint64) {

	unitLength := df.SingleUnitSize()
	rangeLength := uint64(1)
	var err error

	if df.Range != "" {
		if fl.IsAbsoluteAddress(df) {
			_, rangeLength, err = fl.GetAbsoluteAddress(df)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			rangeLength, err = fl.EvaluateExpression(df.Range)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	totalLength := unitLength * rangeLength

	return unitLength, totalLength
}

// returns offset, length from Range "offset:length" syntax. used by images/ico.yml
func (fl *FileLayout) GetAbsoluteAddress(df *value.DataField) (uint64, uint64, error) {

	if !fl.IsAbsoluteAddress(df) {
		log.Fatalf("range is not absolute '%s'", df.Range)
	}

	matches := absoluteRangeExpressionRE.FindAllStringSubmatch(df.Range, -1)
	rangeOffset, err := fl.EvaluateExpression(matches[0][1])
	if err != nil {
		return 0, 0, err
	}
	rangeLength, err := fl.EvaluateExpression(matches[0][2])
	if err != nil {
		return 0, 0, err
	}
	if DEBUG {
		log.Printf("GetAbsoluteAddress: evaluated %s to %d:%d", df.Range, rangeOffset, rangeLength)
	}
	return rangeOffset, rangeLength, nil
}

// returns true if Range is of "offset:length" syntax
func (fl *FileLayout) IsAbsoluteAddress(df *value.DataField) bool {
	matches := absoluteRangeExpressionRE.FindAllStringSubmatch(df.Range, -1)
	return len(matches) == 1 && len(matches[0]) == 3
}

// presents the underlying type as it is known in the template format
func (fl *FileLayout) PresentType(df *value.DataField) string {
	if df.Slice {
		return fmt.Sprintf("%s[]", df.Kind)
	}
	if df.Range != "" {
		unitLength, totalLength := fl.GetLength(df)
		fieldLength := totalLength / unitLength
		return fmt.Sprintf("%s[%d]", df.Kind, fieldLength)
	}
	return df.Kind
}

// replace variables with their values
func (fl *FileLayout) ExpandVariables(s string, df *value.DataField) (string, error) {

	s = strings.Replace(s, "self.offset", fmt.Sprintf("%d", fl.offset), 1)
	s = strings.Replace(s, "FILE_SIZE", fmt.Sprintf("%d", fl.size), 1)

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

		kind, val, err := fl.GetValue(key, df)
		if err != nil {
			s, err := fl.EvaluateExpression(key)
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
