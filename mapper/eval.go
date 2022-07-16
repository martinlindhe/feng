package mapper

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/maja42/goval"
	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/value"
)

var DEBUG_EVAL = false

// an expression failed to evaluate
type EvaluateError struct {
	input string
	msg   string
}

func (e EvaluateError) Error() string {
	return e.input + ": " + e.msg
}

// evaluates a string expression
func (fl *FileLayout) EvaluateStringExpression(in string, df *value.DataField) (string, error) {
	if in == df.Label {
		return "", fmt.Errorf("nothing to eval")
	}
	log.Println("EVAL STR EXPR", in)
	result, err := fl.evaluateExpr(in, df)
	if err != nil {
		return "", err
	}

	switch v := result.(type) {
	case string:
		if DEBUG_EVAL {
			log.Printf("EvaluateStringExpression: %s => %s", in, v)
		}
		return v, nil

	case map[string]interface{}:
		// XXX when no match, return result
		return in, fmt.Errorf("failed to evaluate %s", in)

	default:
		spew.Dump(result)
		panic(fmt.Errorf("unhandled result type %T from %s", result, in))
	}
}

// evaluates a math expression
func (fl *FileLayout) EvaluateExpression(in string, df *value.DataField) (uint64, error) {

	result, err := fl.evaluateExpr(in, df)
	if err != nil {
		return 0, err
	}

	switch v := result.(type) {
	case int:
		if DEBUG_EVAL {
			log.Printf("EvaluateExpression: %s => %d", in, v)
		}
		return uint64(v), nil

	case uint64:
		if DEBUG_EVAL {
			log.Printf("EvaluateExpression: %s => %d", in, v)
		}
		return v, nil

	case bool:
		if v {
			return 1, nil
		} else {
			return 0, nil
		}

	default:
		panic(fmt.Errorf("unhandled result type %T from %s", result, in))
	}
}

func (fl *FileLayout) evaluateExpr(in string, df *value.DataField) (interface{}, error) {
	in = strings.ReplaceAll(in, "OFFSET", fmt.Sprintf("%d", fl.offset))

	in = strings.ReplaceAll(in, "self.", df.Label+".")

	if DEBUG_EVAL {
		feng.Yellow("--- EVALUATING --- %s at %06x (block %s)\n", in, fl.offset, df.Label)
		//spew.Dump(variables)
	}

	// fast path: if "in" looks like decimal number just convert it
	if v, err := strconv.Atoi(in); err == nil {
		return uint64(v), nil
	}

	eval := goval.NewEvaluator()

	variables := make(map[string]interface{})

	for _, layout := range fl.Structs {
		mapped := make(map[string]interface{})

		for _, field := range layout.Fields {
			if !field.Format.Slice && field.Format.Range == "" {
				switch field.Format.Kind {
				case "u8", "u16", "u32", "u64":
					mapped[field.Format.Label] = int(value.AsUint64Raw(field.Value))
				case "i8":
					mapped[field.Format.Label] = int(uint64(int8(value.AsUint64Raw(field.Value))))
				case "i16":
					mapped[field.Format.Label] = int(uint64(int16(value.AsUint64Raw(field.Value))))
				case "i32":
					mapped[field.Format.Label] = int(uint64(int32(value.AsUint64Raw(field.Value))))
				case "i64":
					mapped[field.Format.Label] = int(uint64(int64(value.AsUint64Raw(field.Value))))
				default:
					mapped[field.Format.Label] = field.Present()
				}
			} else {
				mapped[field.Format.Label] = field.Present()
			}

			if DEBUG_EVAL {
				log.Printf("mapped variable %s values %v => %v", field.Format.Label, field.Value, mapped[field.Format.Label])
			}
		}
		mapped["index"] = int(layout.Index)
		variables[layout.Label] = mapped
	}

	// add constants as variables
	for _, constant := range fl.DS.Constants {
		//feng.Green("adding constant %s\n", constant.Field.Label)
		variables[constant.Field.Label] = int(constant.Value)
	}

	functions := make(map[string]goval.ExpressionFunction)

	functions["peek_i32"] = func(args ...interface{}) (interface{}, error) {
		// 1 arg: hex string
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		if v, ok := args[0].(string); ok {
			v, err := value.ParseHexStringToUint64(v)
			if err != nil {
				return nil, err
			}
			offset := int(v)
			if offset >= len(fl.rawData) {
				return 0, fmt.Errorf("out of range %06x", offset)
			}
			val := binary.LittleEndian.Uint32(fl.rawData[offset:]) // XXX endianness
			log.Printf("peek_i32 AT OFFSET %06x: %04x", offset, val)

			return int(val), nil
		}
		return nil, fmt.Errorf("expected string, got %T", args[0])
	}

	functions["peek_i16"] = func(args ...interface{}) (interface{}, error) {
		// 1 arg: hex string
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		if v, ok := args[0].(string); ok {
			v, err := value.ParseHexStringToUint64(v)
			if err != nil {
				return nil, err
			}
			offset := int(v)
			if offset >= len(fl.rawData) {
				return 0, fmt.Errorf("out of range %06x", offset)
			}
			val := binary.LittleEndian.Uint16(fl.rawData[offset:]) // XXX endianness
			log.Printf("peek_i16 AT OFFSET %06x: %04x", offset, val)
			return int(val), nil
		}
		if offset, ok := args[0].(int); ok {
			if offset >= len(fl.rawData) {
				return 0, fmt.Errorf("out of range %06x", offset)
			}
			val := binary.LittleEndian.Uint16(fl.rawData[offset:]) // XXX endianness
			log.Printf("peek_i16 AT OFFSET %06x: %04x", offset, val)
			return int(val), nil
		}
		return nil, fmt.Errorf("expected string, got %T", args[0])
	}

	functions["atoi"] = func(args ...interface{}) (interface{}, error) {
		// 1 arg: string. return integer value
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		if v, ok := args[0].(string); ok {
			res, err := strconv.Atoi(strings.TrimRight(v, " "))
			if err != nil {
				return nil, err
			}
			return res, nil
		}
		return nil, fmt.Errorf("expected string, got %T", args[0])
	}
	functions["abs"] = func(args ...interface{}) (interface{}, error) {
		// 1 arg: integer. return absolute value
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		if i, ok := args[0].(int); ok {
			return int(math.Abs(float64(i))), nil
		}
		return nil, fmt.Errorf("expected int, got %T", args[0])
	}
	functions["offset"] = func(args ...interface{}) (interface{}, error) {
		// 1 arg: name of variable. return its offset as int
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		if s, ok := args[0].(string); ok {
			i, err := fl.GetOffset(s, nil)
			if err != nil {
				panic(err)
			}
			log.Printf("eval offset('%s') => %06x", s, i)
			return i, nil
		}
		return nil, fmt.Errorf("expected string, got %T", args[0])
	}
	functions["len"] = func(args ...interface{}) (interface{}, error) {
		// 1 arg: name of variable. return its data length as int
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		if s, ok := args[0].(string); ok {
			i, err := fl.GetLength(s, nil)
			if err != nil {
				panic(err)
			}
			return i, nil
		}
		return nil, fmt.Errorf("expected string, got %T", args[0])
	}

	functions["not"] = func(args ...interface{}) (interface{}, error) {
		// 2-n arg: reference value, list of values. returns true if 1st number is not any of the others
		if len(args) < 2 {
			return nil, fmt.Errorf("expected at least 2 arguments")
		}
		log.Println("not: starting", args)
		ref := 0
		found := false
		for i, j := range args {
			if v, ok := j.(int); ok {
				if i == 0 {
					ref = v
				} else {
					log.Printf("not: %d == %d  =  %v", v, ref, v == ref)
					if v == ref {
						found = true
					}
				}
			} else {
				return false, fmt.Errorf("expected int, got %T", args[0])
			}
		}
		log.Println("not: returns", !found)
		return !found, nil
	}

	functions["either"] = func(args ...interface{}) (interface{}, error) {
		// 2-n arg: reference value, list of values. returns true if 1st number is in the others
		if len(args) < 2 {
			return nil, fmt.Errorf("expected at least 2 arguments")
		}
		ref := 0
		for i, j := range args {
			if v, ok := j.(int); ok {
				if i == 0 {
					ref = v
				} else {
					log.Printf("in: %d != %d  =  %v", v, ref, v != ref)
					if v == ref {
						return true, nil
					}
				}
			} else {
				return false, fmt.Errorf("expected int, got %T", args[0])
			}
		}
		return false, nil
	}

	result, err := eval.Evaluate(in, variables, functions)
	if err != nil {
		return 0, EvaluateError{input: in, msg: err.Error()}
	}

	return result, nil
}
