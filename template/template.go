package template

import (
	"bytes"
	"fmt"
	"math/bits"
	"reflect"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"

	"github.com/martinlindhe/feng/value"
)

const (
	DEBUG_PATTERNS = false
)

// Template represents the structure of a templates/*.yml file
type Template struct {
	// list of url references for file format
	References []string

	// list of common extensions
	Extensions []string

	// kind of file (archive, image ...)
	Kind string

	// mime type
	Mime string

	// descriptive file format name
	Name string

	// endianness (big, little), can be overridden in a struct declaration
	Endian string

	// if template lacks magic bytes
	NoMagic bool `yaml:"no_magic"`

	// the format uses more than one data file
	MultiFile bool `yaml:"multi_file"`

	// magic id:s
	Magic []Magic

	// file structs
	Structs []yaml.MapItem

	// struct layout
	Layout []string
}

type Constant struct {
	Name  string
	Value []byte
}

// returns all fields with eq or bit subkey pattern matches as constants
func (t *Template) evaluateConstants() ([]Constant, error) {
	res := []Constant{}
	for _, section := range t.Structs {
		constants, err := findStructConstants(&section)
		if err != nil {
			return nil, err
		}
		res = append(res, constants...)
	}

	return res, nil
}

func findStructConstants(c *yaml.MapItem) ([]Constant, error) {
	res := []Constant{}

	if _, ok := c.Value.([]yaml.MapItem); !ok {
		panic(fmt.Sprintf("findStructConstants c.Value is unexpected type '%v'. probably input yaml structs data is malformed (check indentation), or struct has no body", reflect.TypeOf(c.Value)))
	}

	for _, v := range c.Value.([]yaml.MapItem) {
		switch v.Value.(type) {
		case []yaml.MapItem:

			field, err := value.ParseDataField(v.Key.(string))
			if err != nil {
				return nil, err
			}

			if field.IsPatternableUnit() {
				if t, ok := v.Value.([]yaml.MapItem); ok {
					for _, sub := range t {
						m, err := ParseMatchPattern(sub)
						if err != nil {
							log.Print("error (ignoring1):", err)
							continue
						}
						if m.Operation == "eq" || m.Operation == "bit" {
							data, err := value.ParseHexString(m.Pattern)
							if err != nil {
								return nil, err
							}
							df := value.DataField{Label: m.Label, Kind: field.Kind}
							res = append(res, Constant{df.Label, data})
						}
					}
				}
			} else {
				children, err := findStructConstants(&v)
				if err != nil {
					panic(err)
				}

				res = append(res, children...)
			}
		}
	}
	return res, nil
}

// holds a parsed expression
type Expression struct {
	Field value.DataField

	Pattern value.DataPattern

	// represents a branch such as "if <expression>" child nodes
	Children []Expression

	// represents u8/u16/u32/u64 child patterns (eq, bit, default)
	MatchPatterns []MatchPattern
}

// a "structs" node
type evaluatedStruct struct {
	Name string

	Expressions []Expression
}

func (es *Expression) EvaluateMatchPatterns(b []byte, endian string) ([]value.MatchedPattern, error) {
	res := []value.MatchedPattern{}
	if len(es.MatchPatterns) == 0 {
		if DEBUG_PATTERNS {
			log.Printf("MatchPattern: gave up early on %s", es.Field.Label)
		}
		return res, nil
	}
	invalidIfNoMatch := false

	if DEBUG_PATTERNS {
		log.Printf("MatchPattern: looking for %#v", b)
	}

	for _, mp := range es.MatchPatterns {
		if DEBUG_PATTERNS {
			log.Printf("--- %#v", mp)
		}

		switch mp.Operation {
		case "bit":
			bitmaskSlice, err := value.ParseHexString(mp.Pattern)
			if err != nil {
				return nil, err
			}
			actual := value.AsUint64(es.Field.Kind, b)
			bitmask := value.AsUint64(es.Field.Kind, bitmaskSlice)
			masked := bitmask & actual
			ones := bits.OnesCount(uint(bitmask))
			shift := bits.TrailingZeros64(bitmask)
			val := masked >> shift

			if DEBUG_PATTERNS {
				log.Printf("--- %s %s: bitmask %02x %08b on value %02x %08b == res %02x %08b",
					mp.Operation, es.Field.Kind, bitmask, bitmask, actual, actual, val, val)
			}
			res = append(res, value.MatchedPattern{
				Label:     mp.Label,
				Operation: mp.Operation,
				Value:     b,
				Parsed:    fmt.Sprintf("%d", val),
				Index:     int8(shift),
				Size:      int8(ones)})

		case "eq":
			patternData, err := value.ParseHexString(mp.Pattern)
			if err != nil {
				return nil, err
			}
			match := bytes.Compare(patternData, b)

			if DEBUG_PATTERNS {
				log.Printf("--- %s %s: %v == %v is %v (%s)", mp.Operation, es.Field.Kind, b, patternData, match, mp.Label)
			}
			if match == 0 {
				res = append(res, value.MatchedPattern{
					Label:     mp.Label,
					Operation: mp.Operation,
					Value:     b,
				})
			}

		case "default":
			if mp.Label != "invalid" {
				return nil, fmt.Errorf("invalid default value '%s'", mp.Label)
			}
			invalidIfNoMatch = true

		default:
			log.Fatal().Msgf("unhandled pattern match operation '%s'", mp.Operation)
		}
	}
	if invalidIfNoMatch && len(res) == 0 {
		// if we don't find any patterns, return error
		return nil, ValidationError{fmt.Sprintf("value %v for %s is not valid", b, es.Field.Label)}
	}
	return res, nil
}

// the input file failed to match a required marker
type ValidationError struct {
	Message string
}

func (r ValidationError) Error() string {
	return r.Message
}

func (t *Template) evaluateStructs() ([]evaluatedStruct, error) {
	res := []evaluatedStruct{}
	for _, c := range t.Structs {
		es, err := parseStruct(&c)
		if err != nil {
			return nil, err
		}
		res = append(res, es)
	}
	return res, nil
}

// parses a "struct" child with all their child nodes
func parseStruct(c *yaml.MapItem) (evaluatedStruct, error) {

	key := c.Key.(string)
	es := evaluatedStruct{Name: key}

	for _, v := range c.Value.([]yaml.MapItem) {
		field, err := value.ParseDataField(v.Key.(string))
		if err != nil {
			log.Printf("TEMPLATE ERROR: cant parse field '%s': %v", v.Key.(string), err)
			return es, err
		}

		var expr Expression
		switch val := v.Value.(type) {
		case []yaml.MapItem:
			if field.IsPatternableUnit() {
				matchPatterns, err := parseMatchPatterns(val)
				if err != nil {
					return es, err
				}
				expr = Expression{field, value.DataPattern{}, []Expression{}, matchPatterns}

			} else {
				// evaluate all child nodes (if <expression>)
				children, err := parseStruct(&v)
				if err != nil {
					return es, err
				}
				expr = Expression{field, value.DataPattern{}, children.Expressions, []MatchPattern{}}
			}

		case string:
			switch field.Kind {
			case "endian", "data", "label", "offset", "filename", "parse", "until", "encryption", "import":
				pattern := value.DataPattern{Known: true, Value: val}
				expr = Expression{field, pattern, []Expression{}, []MatchPattern{}}

			default:
				pattern, err := value.ParseDataPattern(val)
				if err != nil {
					log.Printf("%#v", field)
					log.Fatal().Msgf("TEMPLATE ERROR: cant parse pattern '%s': %v", val, err)
					return es, err
				}
				expr = Expression{field, pattern, []Expression{}, []MatchPattern{}}
			}

		default:
			log.Fatal().Msgf("cant handle type '%T' in '%#v'", val, v)
		}
		es.Expressions = append(es.Expressions, expr)
	}

	return es, nil
}

type MatchPattern struct {
	// pattern description
	Label string

	// eq, bit
	Operation string

	Pattern string
}

func parseMatchPatterns(mi []yaml.MapItem) ([]MatchPattern, error) {
	res := []MatchPattern{}

	for _, item := range mi {
		p, err := ParseMatchPattern(item)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed")
		}
		res = append(res, *p)
	}

	return res, nil
}

// parses a key-value pattern field such as "eq 025f: TYPE_ONE"
func ParseMatchPattern(item yaml.MapItem) (*MatchPattern, error) {

	p := MatchPattern{}

	key := strings.TrimSpace(item.Key.(string))
	value := strings.TrimSpace(item.Value.(string))

	parts := strings.SplitN(key, " ", 2)
	if len(parts) <= 0 || len(parts) > 2 {
		return nil, fmt.Errorf("unexpected match pattern: '%s'", key)
	}

	switch parts[0] {
	case "eq", "bit", "default":
		p.Operation = parts[0]
		if len(parts) >= 2 {
			p.Pattern = parts[1]
		}
		p.Label = value

	default:
		log.Fatal().Msgf("evaluateMatchPatterns: unrecognized form '%s': %s", parts[0], key)
	}
	return &p, nil
}

func (t *Template) evaluateLayout() ([]value.DataField, error) {

	res := []value.DataField{}

	for _, s := range t.Layout {
		key, err := value.ParseDataField(s)
		//log.Println(s, key)
		if err != nil {
			return nil, err
		}
		res = append(res, key)
	}

	return res, nil
}

type Magic struct {
	Offset HexStringU64
	Match  HexString
	Endian string
}
type HexString []byte

// Implements the Unmarshaler interface of the yaml pkg.
func (e *HexString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		v, err := value.ParseHexString(s)
		if err != nil {
			return err
		}
		*e = HexString(v)
		return err
	}
	return nil
}

type HexStringU64 uint64

// Implements the Unmarshaler interface of the yaml pkg.
func (e *HexStringU64) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		if s[0:2] == "0x" {
			s = s[2:]
		}
		v, err := value.ParseHexStringToUint64(s)
		if err != nil {
			return err
		}
		*e = HexStringU64(v)
		return err
	}
	return nil
}
