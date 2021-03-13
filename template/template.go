package template

import (
	"fmt"
	"log"
	"math/bits"
	"strings"

	"github.com/martinlindhe/feng/value"
	"gopkg.in/yaml.v2"
)

const (
	DEBUG = true
)

// Template represents a templates/*.yml file
type Template struct {
	// list of url references for file format
	References []string

	// list of common extensions
	Extensions []string

	// kind of file (archive, image ...)
	Kind string

	// mime type
	Mime string

	// constants
	Constants []yaml.MapItem

	// file structs
	Structs []yaml.MapItem

	// struct layout
	Layout []string
}

type EvaluatedConstant struct {
	Field value.DataField
	Value []byte
}

func (t *Template) evaluateConstants() ([]EvaluatedConstant, error) {
	res := []EvaluatedConstant{}
	for _, c := range t.Constants {
		key, err := value.ParseDataField(c.Key.(string))
		if err != nil {
			return nil, err
		}
		val, err := value.ParseDataString(c.Value.(string))
		if err != nil {
			return nil, err
		}
		res = append(res, EvaluatedConstant{key, val})
	}

	return res, nil
}

// holds a parsed expression
type expression struct {
	Field value.DataField

	Pattern value.DataPattern

	// represents a branch such as "if <expression>" child nodes
	Children []expression

	// represents u8/u16/u32/u64 child patterns (eq, bit, default)
	MatchPatterns []MatchPattern
}

// a "structs" node
type evaluatedStruct struct {
	Name string

	Expressions []expression
}

func (es *expression) EvaluateMatchPatterns(b []byte) ([]value.MatchedPattern, error) {
	res := []value.MatchedPattern{}
	invalidIfNoMatch := false
	actual := value.AsUint64(es.Field.Kind, b)

	for _, mp := range es.MatchPatterns {
		if DEBUG {
			log.Printf("--- MatchPattern: %#v", mp)
		}

		switch mp.Operation {
		case "bit":
			bitmaskSlice, err := value.ParseDataString(mp.Pattern)
			if err != nil {
				return nil, err
			}
			bitmask := value.AsUint64(es.Field.Kind, bitmaskSlice)
			masked := bitmask & actual
			shift := bits.TrailingZeros64(masked)
			val := masked >> shift

			if DEBUG {
				log.Printf("--- MatchPattern %s %s: bitmask %02x %08b on value %02x %08b == res %02x %08b", mp.Operation, es.Field.Kind, bitmask, bitmask, actual, actual, val, val)
			}
			res = append(res, value.MatchedPattern{Label: mp.Label, Operation: mp.Operation, Value: val})

		case "eq":
			patternData, err := value.ParseDataString(mp.Pattern)
			if err != nil {
				return nil, err
			}
			pattern := value.AsUint64(es.Field.Kind, patternData)
			match := actual == pattern

			if DEBUG {
				log.Printf("--- MatchPattern %s %s: %08x == %08x is %v", mp.Operation, es.Field.Kind, actual, pattern, match)
			}
			if match {
				res = append(res, value.MatchedPattern{Label: mp.Label, Operation: mp.Operation, Value: actual})
			}

		case "default":
			if mp.Label != "invalid" {
				return nil, fmt.Errorf("invalid default value '%s'", mp.Label)
			}
			invalidIfNoMatch = true

		default:
			log.Fatalf("unhandled matchpattern operation '%s'", mp.Operation)
		}
	}
	if invalidIfNoMatch && len(res) == 0 {
		// if we don't find any patterns, return error
		return nil, fmt.Errorf("value %08x for %s is not valid", actual, es.Field.Label)
	}
	return res, nil
}

func (t *Template) evaluateStructs() ([]evaluatedStruct, error) {
	res := []evaluatedStruct{}
	for _, c := range t.Structs {

		es, err := evaluateStruct(&c)
		if err != nil {
			return nil, err
		}
		res = append(res, es)
	}

	return res, nil
}

// evaluates a "struct" child with all their child nodes
func evaluateStruct(c *yaml.MapItem) (evaluatedStruct, error) {

	key := c.Key.(string)
	es := evaluatedStruct{Name: key}

	for _, v := range c.Value.([]yaml.MapItem) {
		field, err := value.ParseDataField(v.Key.(string))
		if err != nil {
			log.Fatalf("ERROR IN TEMPLATE: cant parse field '%s': %v", v.Key.(string), err)
			return es, err
		}

		var expr expression

		switch val := v.Value.(type) {
		case []yaml.MapItem:
			// if current node is u8, u16, u32 or u64, childs must be pattern matchers (bit / eq)
			if field.IsPatternableUnit() {
				matchPatterns, err := evaluateMatchPatterns(val)
				if err != nil {
					return es, err
				}
				expr = expression{field, value.DataPattern{}, []expression{}, matchPatterns}

			} else {
				// evaluate all child nodes (if <expression>)
				children, err := evaluateStruct(&v)
				if err != nil {
					return es, err
				}
				expr = expression{field, value.DataPattern{}, children.Expressions, []MatchPattern{}}
			}

		case string:
			if field.Kind == "endian" {
				pattern := value.DataPattern{Known: true, Value: val}
				expr = expression{field, pattern, []expression{}, []MatchPattern{}}
			} else {
				pattern, err := value.ParseDataPattern(val)
				if err != nil {
					log.Fatalf("TEMPLATE ERROR: cant parse pattern '%s': %v", val, err)
					return es, err
				}
				expr = expression{field, pattern, []expression{}, []MatchPattern{}}
			}

		default:
			log.Fatalf("evaluateStructs: cant handle type '%T' in '%#v'", val, v)
		}
		if DEBUG {
			log.Printf("evaluateStruct: appending %+v", expr)
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

func evaluateMatchPatterns(mi []yaml.MapItem) ([]MatchPattern, error) {
	res := []MatchPattern{}

	for _, item := range mi {
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
			log.Fatalf("evaluateMatchPatterns: unrecognized form '%s': %s", parts[0], key)
		}

		res = append(res, p)
	}

	return res, nil
}

func (t *Template) evaluateLayout() ([]value.DataField, error) {

	res := []value.DataField{}

	for _, s := range t.Layout {
		key, err := value.ParseDataField(s)
		if err != nil {
			return nil, err
		}
		res = append(res, key)
	}

	return res, nil
}
