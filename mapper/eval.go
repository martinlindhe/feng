package mapper

import (
	"fmt"
	"log"
	"math"
	"strconv"

	"github.com/maja42/goval"
	"github.com/martinlindhe/feng/value"
)

type EvaluateError struct {
	input string
	msg   string
}

func (e EvaluateError) Error() string {
	return e.input + ": " + e.msg
}

func (fl *FileLayout) EvaluateExpression(s string) (uint64, error) {

	eval := goval.NewEvaluator()

	variables := make(map[string]interface{})
	for _, layout := range fl.Structs {
		mapped := make(map[string]interface{})
		for _, field := range layout.Fields {

			if len(field.MatchedPatterns) > 0 {
				children := make(map[string]interface{})
				for _, child := range field.MatchedPatterns {
					children[child.Label] = int(child.Value)
				}
				mapped[field.Format.Label] = children
			} else {
				value := value.Present(field.Format, field.Value)
				if i, err := strconv.ParseInt(value, 10, 64); err == nil {
					mapped[field.Format.Label] = int(i)
				} else {
					mapped[field.Format.Label] = value
				}
			}
		}
		variables[layout.Label] = mapped
	}
	if DEBUG {
		log.Printf("variables: %#v", variables)
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
		return nil, fmt.Errorf("expected int")
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
		return nil, fmt.Errorf("expected string")
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
		return nil, fmt.Errorf("expected string")
	}

	result, err := eval.Evaluate(s, variables, functions)
	if err != nil {
		return 0, EvaluateError{input: s, msg: err.Error()}
	}

	switch v := result.(type) {
	case int:
		if DEBUG {
			log.Printf("EvaluateExpression: %s => %d", s, v)
		}
		return uint64(v), nil
	}
	return 0, fmt.Errorf("unhandled result type %T", result)
}
