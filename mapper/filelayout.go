package mapper

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

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
	variableExpressionRE = regexp.MustCompile(`\(([\w .]+)\)`)
)

// decodes simple value types for presentation
func (field *Field) Present() string {
	kind := field.Format.PresentType()
	if field.Format.SingleUnitSize() > 1 {
		if field.Endian == "little" {
			kind += " le"
		} else {
			kind += " be"
		}
	}

	fieldValue := value.Present(field.Format, field.Value)

	res := fmt.Sprintf("  [%06x] %-30s %-10s %-10s %-20s\n",
		field.Offset, field.Format.Label, kind, fieldValue, fmt.Sprintf("% 02x", field.Value))

	for _, child := range field.MatchedPatterns {
		res += fmt.Sprintf("           - %-28s %-10s %d\n", child.Label, child.Operation, child.Value)
	}
	return res
}

func (fl *FileLayout) Present() {
	for _, layout := range fl.Structs {
		fmt.Printf("%s\n", layout.Label)
		for _, field := range layout.Fields {
			fmt.Print(field.Present())
		}
	}
}

func (fl *FileLayout) GetStruct(name string) (*Struct, error) {
	for _, str := range fl.Structs {
		if DEBUG {
			log.Printf("GetStruct: searching struct %s (%d fields)", str.Label, len(str.Fields))
		}
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
		log.Printf("GetValue: searching for %s", s)
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
			log.Printf("comparing field %s to %s", field.Format.Label, fieldName)
		}
		if field.Format.Label == fieldName {
			if !field.Format.IsSimpleUnit() || childName == "" {
				return field.Format.Kind, field.Value, nil
			}

			for _, child := range field.MatchedPatterns {
				if child.Label == childName {
					val := value.U64toBytesBigEndian(child.Value, field.Format.SingleUnitSize())
					if DEBUG {
						log.Printf("matched pattern %s = %d", child.Label, val)
					}
					return field.Format.Kind, val, nil
				}
			}
		}
	}

	return "", nil, fmt.Errorf("GetValue: '%s' not found", s)
}

// replace variables with their values
func (fl *FileLayout) ExpandVariables(s string, df *value.DataField) (string, error) {
	if !strings.Contains(s, "(") && !strings.Contains(s, ")") {
		s = "(" + s + ")"
	}

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
	matches := variableExpressionRE.FindStringSubmatch(s)
	if len(matches) == 0 {
		return s, nil
	}

	idx := variableExpressionRE.FindStringSubmatchIndex(s)

	key := matches[1]

	// don't resolve if integer-like
	if isIntegerString(key) {
		return s[0:idx[0]] + key + s[idx[1]:], nil
	}

	kind, val, err := fl.GetValue(key, df)
	if err != nil {
		return "", err
	}

	log.Printf("ExpandVariables: MATCHED %s to %s %v", key, kind, val)

	i := value.AsUint64(kind, val)

	return s[0:idx[0]] + fmt.Sprintf("%d", i) + s[idx[1]:], nil
}

// returns true if string represents an integer
func isIntegerString(s string) bool {
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	return false
}
