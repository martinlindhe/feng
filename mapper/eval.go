package mapper

import (
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

// an expression failed to evaluate
type EvaluateError struct {
	input string
	msg   string
}

func (e EvaluateError) Error() string {
	return e.input + ": " + e.msg
}

func (fl *FileLayout) EvaluateExpression(in string) (uint64, error) {

	in = strings.ReplaceAll(in, "OFFSET", fmt.Sprintf("%d", fl.offset))

	// fast path: if "in" looks like decimal number just convert it
	if v, err := strconv.Atoi(in); err == nil {
		return uint64(v), nil
	}

	eval := goval.NewEvaluator()

	variables := make(map[string]interface{})

	for _, layout := range fl.Structs {
		mapped := make(map[string]interface{})
		for _, field := range layout.Fields {
			val := uint64(0)
			if !field.Format.Slice && field.Format.Range == "" {
				switch field.Format.Kind {
				case "u8", "u16", "u32", "u64":
					val = value.AsUint64Raw(field.Value)
				case "i8":
					val = uint64(int8(value.AsUint64Raw(field.Value)))
				case "i16":
					val = uint64(int16(value.AsUint64Raw(field.Value)))
				case "i32":
					val = uint64(int32(value.AsUint64Raw(field.Value)))
				case "i64":
					val = uint64(int64(value.AsUint64Raw(field.Value)))

				default:
					i, err := strconv.ParseInt(field.Present(), 10, 64)
					if err == nil {
						val = uint64(i)
					}
				}
			}

			if DEBUG {
				//log.Printf("mapped variable %s to %v => %d", field.Format.Label, field.Value, int(val))
			}
			mapped[field.Format.Label] = int(val)
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

	functions["abs"] = func(args ...interface{}) (interface{}, error) {
		// 1 arg: integer. return absolute value
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		i, ok := args[0].(int)
		if ok {
			return int(math.Abs(float64(i))), nil
		}
		return nil, fmt.Errorf("expected int, got %T", args[0])
	}
	functions["offset"] = func(args ...interface{}) (interface{}, error) {
		// 1 arg: name of variable. return its offset as int
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		s, ok := args[0].(string)
		if ok {
			i, err := fl.GetOffset(s, nil)
			if err != nil {
				panic(err)
			}
			return i, nil
		}
		return nil, fmt.Errorf("expected string, got %T", args[0])
	}
	functions["len"] = func(args ...interface{}) (interface{}, error) {
		// 1 arg: name of variable. return its offset as int
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		s, ok := args[0].(string)
		if ok {
			i, err := fl.GetLength(s, nil)
			if err != nil {
				panic(err)
			}
			return i, nil
		}
		return nil, fmt.Errorf("expected string, got %T", args[0])
	}

	if DEBUG {
		feng.Yellow("--- EVALUATING --- %s\n", in)
		spew.Dump(variables)
		feng.Yellow("---\n")
	}

	result, err := eval.Evaluate(in, variables, functions)
	if err != nil {
		return 0, EvaluateError{input: in, msg: err.Error()}
	}

	switch v := result.(type) {
	case int:
		if DEBUG {
			log.Printf("EvaluateExpression: %s => %d", in, v)
		}
		return uint64(v), nil

	case bool:
		if v {
			return 1, nil
		} else {
			return 0, nil
		}
	}
	panic(fmt.Errorf("unhandled result type %T", result))
	return 0, fmt.Errorf("unhandled result type %T", result)
}
