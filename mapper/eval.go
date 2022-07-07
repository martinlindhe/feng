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

var DEBUG_EVAL = false

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
			}

			if DEBUG_EVAL {
				//log.Printf("mapped variable %s to %v => %v", field.Format.Label, field.Value, mapped[field.Format.Label])
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

	if DEBUG_EVAL {
		// XXX seems Align field is not yet available as variable in eval.go for some reason??!
		// it should have been added already
		feng.Yellow("--- EVALUATING --- %s at %06x\n", in, fl.offset)
		spew.Dump(variables)
		if in == `Segment_1.Align == "II"` {

			//panic("wa")
		}
		feng.Yellow("---\n")
	}

	result, err := eval.Evaluate(in, variables, functions)
	if err != nil {
		return 0, EvaluateError{input: in, msg: err.Error()}
	}

	switch v := result.(type) {
	case int:
		if DEBUG_EVAL {
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
