package mapper

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/martinlindhe/feng/value"
)

// parsed file data from a template "layout"
type FileLayout struct {
	Structs []FileStruct
}

// parsed file data section from a template "struct"
type FileStruct struct {
	Label string

	Fields []fileField
}

type fileField struct {
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

// decodes simple value types for presentation
func (ff *fileField) Present() string {
	if ff.Format.Slice || ff.Format.Range != "" {
		return ""
	}
	v := value.AsUint64(ff.Format.Kind, ff.Value)
	return fmt.Sprintf("%d", v)
}

func (fl *FileLayout) Present() {
	for _, layout := range fl.Structs {
		fmt.Printf("%s\n", layout.Label)

		for _, field := range layout.Fields {
			kind := field.Format.PresentType()
			if field.Format.SingleUnitSize() > 1 {
				if field.Endian == "little" {
					kind += " le"
				} else {
					kind += " be"
				}
			}

			fmt.Printf("  [%06x] %-30s %-10s %-10s %-20s\n",
				field.Offset, field.Format.Label, kind, field.Present(), fmt.Sprintf("% 02x", field.Value))

			for _, child := range field.MatchedPatterns {
				fmt.Printf("           - %-28s %-10s %d\n", child.Label, child.Operation, child.Value)
			}
		}
	}
}

// finds the first field named `structName`.`fieldName`
// returns: kind, bytes, error
func (fl *FileLayout) GetValue(structName, fieldName string) (string, []byte, error) {
	if DEBUG {
		log.Printf("searching for %s.%s", structName, fieldName)
	}
	for _, struct_ := range fl.Structs {
		if DEBUG {
			log.Printf("comparing struct %s to %s", struct_.Label, structName)
		}
		if struct_.Label == structName {
			return struct_.GetValue(fieldName)
		}
	}
	return "", nil, fmt.Errorf("struct not found")
}

// returns: kind, bytes, error
func (fs *FileStruct) GetValue(fieldName string) (string, []byte, error) {
	childName := ""
	separator := strings.Index(fieldName, ".")
	if separator != -1 {
		childName = fieldName[separator+1:]
		fieldName = fieldName[0:separator]
	}

	if DEBUG {
		log.Printf("searching for '%s.%s.%s'", fs.Label, fieldName, childName)
	}

	for _, field := range fs.Fields {
		if DEBUG {
			log.Printf("comparing field %s to %s", field.Format.Label, fieldName)
		}
		if field.Format.Label == fieldName {
			if !field.Format.IsSimpleUnit() {
				return "", nil, fmt.Errorf("type '%s' cannot be used in IF-statement", field.Format.PresentType())
			}
			if childName == "" {
				return field.Format.Kind, field.Value, nil
			}

			for _, child := range field.MatchedPatterns {
				if DEBUG {
					log.Printf("comparing matched pattern %s to %s", child.Label, childName)
				}
				if child.Label == childName {
					val := value.U64toBytesBigEndian(child.Value, field.Format.SingleUnitSize())
					return field.Format.Kind, val, nil
				}
			}
		}
	}
	return "", nil, fmt.Errorf("field not found")
}

// replace variables with their values
func (fs *FileStruct) ExpandVariables(s string) string {
	log.Printf("ExpandVariables: %s", s)

	for {
		expanded := fs.expandVariable(s)
		if expanded == s {
			break
		}
		log.Printf("ExpandVariables: %s => %s", s, expanded)
		s = expanded
	}

	return s
}

func (fs *FileStruct) expandVariable(s string) string {
	variableExpressionRE := regexp.MustCompile(`\(([\w .]+)\)`)

	matches := variableExpressionRE.FindStringSubmatch(s)
	if len(matches) == 0 {
		return s
	}

	// XXX 1 expansion, then recurse until no difference

	idx := variableExpressionRE.FindStringSubmatchIndex(s)

	kind, val, err := fs.GetValue(matches[1])
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("ExpandVariables: MATCHED %s to %s %v", matches[1], kind, val)

	i := value.AsUint64(kind, val)

	return s[0:idx[0]] + fmt.Sprintf("%d", i) + s[idx[1]:]
}
