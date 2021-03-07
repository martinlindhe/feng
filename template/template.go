package template

import (
	"fmt"
	"log"
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

type evaluatedConstant struct {
	Field value.DataField
	Value []byte
}

func (t *Template) evaluateConstants() ([]evaluatedConstant, error) {
	res := []evaluatedConstant{}
	for _, c := range t.Constants {
		key, err := value.ParseDataField(c.Key.(string))
		if err != nil {
			return nil, err
		}
		val, err := value.ParseDataString(c.Value.(string))
		if err != nil {
			return nil, err
		}
		res = append(res, evaluatedConstant{key, val})
	}

	return res, nil
}

// holds a parsed expression
type expression struct {
	Field value.DataField

	Pattern value.DataPattern

	// represents a branch such as "if <expression>" child nodes
	children []expression

	// represents u8/u16/u32/u64 child patterns (eq, bit, default)
	matchPatterns []matchPattern
}

// a "structs" node
type evaluatedStruct struct {
	Name string

	Expressions []expression
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

		switch val := v.Value.(type) {
		case []yaml.MapItem:
			// if current node is u8, u16, u32 or u64, childs must be pattern matchers (bit / eq)
			if field.IsPatternableUnit() {
				matchPatterns, err := evaluateMatchPatterns(val)
				if err != nil {
					return es, err
				}
				es.Expressions = append(es.Expressions, expression{field, value.DataPattern{}, []expression{}, matchPatterns})

			} else {

				// evaluate all child nodes (if <expression>)
				children, err := evaluateStruct(&v)
				if err != nil {
					return es, err
				}
				es.Expressions = append(es.Expressions, expression{field, value.DataPattern{}, children.Expressions, []matchPattern{}})
			}

		case string:
			if field.Kind == "endian" {
				pattern := value.DataPattern{Known: true, Value: val}
				es.Expressions = append(es.Expressions, expression{field, pattern, []expression{}, []matchPattern{}})
			} else {
				pattern, err := value.ParseDataPattern(val)
				if err != nil {
					log.Fatalf("TEMPLATE ERROR: cant parse pattern '%s': %v", val, err)
					return es, err
				}
				es.Expressions = append(es.Expressions, expression{field, pattern, []expression{}, []matchPattern{}})
			}

		default:
			log.Fatalf("evaluateStructs: cant handle %T", val)
		}
	}

	return es, nil
}

type matchPattern struct {
	// eq, bit
	operation string

	pattern string // XXX how to store evaluated ?!

	// pattern description
	label string
}

func evaluateMatchPatterns(mi []yaml.MapItem) ([]matchPattern, error) {
	res := []matchPattern{}

	for _, item := range mi {
		p := matchPattern{}

		key := strings.TrimSpace(item.Key.(string))
		value := strings.TrimSpace(item.Value.(string))

		parts := strings.SplitN(key, " ", 2)
		if len(parts) <= 0 || len(parts) > 2 {
			return nil, fmt.Errorf("unexpected match pattern: '%s'", key)
		}

		switch parts[0] {
		case "eq", "bit", "default":
			p.operation = parts[0]
			if len(parts) >= 2 {
				p.pattern = parts[1] // XXX should pattern be evaluated here?
			}
			p.label = value

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
