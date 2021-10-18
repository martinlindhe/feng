package mapper

import (
	"fmt"
	"log"
	"math"
	"strconv"

	"github.com/maja42/goval"
	"github.com/martinlindhe/feng/value"
)

func (fl *FileLayout) EvaluateExpression(s string) (uint64, error) {

	eval := goval.NewEvaluator()

	variables := make(map[string]interface{})
	for _, layout := range fl.Structs {
		mapped := make(map[string]interface{})
		for _, field := range layout.Fields {
			value := value.Present(field.Format, field.Value)
			if i, err := strconv.ParseInt(value, 10, 64); err == nil {
				mapped[field.Format.Label] = int(i)
			} else {
				mapped[field.Format.Label] = value
			}
		}
		variables[layout.Label] = mapped
	}

	functions := make(map[string]goval.ExpressionFunction)

	functions["abs"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly 1 argument")
		}
		i, ok := args[0].(int)
		if ok {
			return int(math.Abs(float64(i))), nil
		}
		return nil, fmt.Errorf("expected string")
	}

	result, err := eval.Evaluate(s, variables, functions)
	if err != nil {
		return 0, fmt.Errorf("cant evaluate '%s': %v", s, err)
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
