package mapper

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/maja42/goval"
	"github.com/rs/zerolog/log"

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

	log.Debug().Msgf("EVAL STR EXPR %s", in)

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
func (fl *FileLayout) EvaluateExpression(in string, df *value.DataField) (int64, error) {
	result, err := fl.evaluateExpr(in, df)
	if err != nil {
		return 0, err
	}

	switch v := result.(type) {
	case int:
		if DEBUG_EVAL {
			log.Printf("EvaluateExpression: %s => %d", in, v)
		}
		return int64(v), nil

	case uint64:
		if DEBUG_EVAL {
			log.Printf("EvaluateExpression: %s => %d", in, v)
		}
		return int64(v), nil

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

func (fl *FileLayout) evalPeekI32(args ...interface{}) (interface{}, error) {
	// 1 arg: hex string
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if v, ok := args[0].(string); ok {
		v, err := value.ParseHexStringToUint64(v)
		if err != nil {
			return nil, err
		}
		offset := int64(v)
		if offset >= int64(fl.size) {
			return 0, fmt.Errorf("out of range %06x", offset)
		}

		val, _ := fl.peekU32(offset)

		if DEBUG_EVAL {
			log.Printf("peek_i32 AT OFFSET %06x: %04x", offset, int(val))
		}
		return int(val), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

func (fl *FileLayout) peekU32(offset int64) (uint32, error) {
	prevOffset, err := fl._f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	_, _ = fl._f.Seek(offset, io.SeekStart)
	buf := make([]byte, 4)
	_, _ = fl._f.Read(buf)
	val := binary.LittleEndian.Uint32(buf)
	_, _ = fl._f.Seek(prevOffset, io.SeekStart)
	return val, nil
}

func (fl *FileLayout) peekU16(offset int64) (uint16, error) {
	prevOffset, err := fl._f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	_, _ = fl._f.Seek(offset, io.SeekStart)
	buf := make([]byte, 2)
	_, _ = fl._f.Read(buf)
	val := binary.LittleEndian.Uint16(buf)
	_, _ = fl._f.Seek(prevOffset, io.SeekStart)
	return val, nil
}

func (fl *FileLayout) peekU8(offset int64) (uint8, error) {
	prevOffset, err := fl._f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	_, _ = fl._f.Seek(offset, io.SeekStart)
	buf := make([]byte, 1)
	_, _ = fl._f.Read(buf)
	_, _ = fl._f.Seek(prevOffset, io.SeekStart)
	return buf[0], nil
}

// returns a slice of bytes from file, otherwise unmodified
func (fl *FileLayout) peekBytes(offset int64, size int64) ([]uint8, error) {
	prevOffset, err := fl._f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	_, _ = fl._f.Seek(offset, io.SeekStart)
	buf := make([]byte, size)
	_, _ = fl._f.Read(buf)
	_, _ = fl._f.Seek(prevOffset, io.SeekStart)
	return buf, nil
}

func (fl *FileLayout) evalPeekI16(args ...interface{}) (interface{}, error) {
	// 1 arg: hex string
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if v, ok := args[0].(string); ok {
		v, err := value.ParseHexStringToUint64(v)
		if err != nil {
			return nil, err
		}
		offset := int64(v)
		if offset >= int64(fl.size) {
			return 0, fmt.Errorf("out of range %06x", offset)
		}

		val, _ := fl.peekU16(offset)

		if DEBUG_EVAL {
			log.Printf("peek_i16 AT OFFSET %06x: %04x", offset, val)
		}
		return int(val), nil
	}
	if offset, ok := args[0].(int64); ok {
		if offset >= int64(fl.size) {
			return 0, fmt.Errorf("out of range %06x", offset)
		}
		val, _ := fl.peekU16(offset)

		if DEBUG_EVAL {
			log.Printf("peek_i16 AT OFFSET %06x: %04x", offset, val)
		}
		return int(val), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

func (fl *FileLayout) evalPeekI8(args ...interface{}) (interface{}, error) {
	// 1 arg: hex string
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if v, ok := args[0].(string); ok {
		v, err := value.ParseHexStringToUint64(v)
		if err != nil {
			return nil, err
		}
		offset := int64(v)
		if offset >= int64(fl.size) {
			return 0, fmt.Errorf("out of range %06x", offset)
		}
		val, _ := fl.peekU8(offset)

		if DEBUG_EVAL {
			log.Printf("peek_i8 AT OFFSET %06x: %04x", offset, val)
		}
		return int(val), nil
	}
	if offset, ok := args[0].(int64); ok {
		if offset >= int64(fl.size) {
			return 0, fmt.Errorf("out of range %06x", offset)
		}
		val, _ := fl.peekU8(offset)
		if DEBUG_EVAL {
			log.Printf("peek_i8 AT OFFSET %06x: %02x", offset, val)
		}
		return int(val), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

func (fl *FileLayout) evalAtoi(args ...interface{}) (interface{}, error) {
	// convert alphanumeric string to int
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

func (fl *FileLayout) evalOtoi(args ...interface{}) (interface{}, error) {
	// convert octal numeric string to int
	// 1 arg: string. return integer value
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if v, ok := args[0].(string); ok {
		v = presentStringValue(v)
		if v == "" {
			return 0, nil
		}
		res, err := strconv.ParseInt(v, 8, 64)
		if err != nil {
			return nil, err
		}
		return int(res), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

func (fl *FileLayout) evalCeil(args ...interface{}) (interface{}, error) {
	// returns ceil of float64
	// 1 arg: int. return integer value
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if v, ok := args[0].(float64); ok {
		res := math.Ceil(float64(v))
		return int(res), nil
	}
	return nil, fmt.Errorf("expected int, got %T", args[0])
}

func (fl *FileLayout) evalAbs(args ...interface{}) (interface{}, error) {
	// 1 arg: integer. return absolute value
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if i, ok := args[0].(int); ok {
		return int(math.Abs(float64(i))), nil
	}
	return nil, fmt.Errorf("expected int, got %T", args[0])
}

func (fl *FileLayout) evalAlignment(args ...interface{}) (interface{}, error) {
	// 2 args: 1) size value, 2) alignment. return the alignment needed for size value to fit into "alignment"
	// example: value 4, align 4. returns 0
	// example: value 4, align 8. returns 4
	if len(args) != 2 {
		return nil, fmt.Errorf("expected exactly 2 argument")
	}
	v1, ok := args[0].(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", args[0])
	}
	v2, ok := args[1].(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", args[1])
	}

	i := (v2 - (v1 % v2)) % v2
	if DEBUG_EVAL {
		log.Printf("aligned %d, %d => %d", v1, v2, i)
	}
	return i, nil
}

func (fl *FileLayout) evalOffset(args ...interface{}) (interface{}, error) {
	// 1 arg: name of variable. return its offset as int
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if s, ok := args[0].(string); ok {
		i, err := fl.GetOffset(s, nil)
		if err != nil {
			panic(err)
		}
		if DEBUG_EVAL {
			log.Printf("eval offset('%s') => %06x", s, i)
		}
		return i, nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

func (fl *FileLayout) evalLen(args ...interface{}) (interface{}, error) {
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

func (fl *FileLayout) evalSevenBitString(args ...interface{}) (interface{}, error) {
	// 1 arg: name of variable. return manipulated string value
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if s, ok := args[0].(string); ok {
		_, val, err := fl.GetValue(s, nil)
		if err != nil {
			panic(err)
		}

		// for each byte, remove high bit.
		out := []byte{}
		for _, c := range val {
			out = append(out, c&0x7f)
		}

		return string(out), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

func (fl *FileLayout) evalCleanString(args ...interface{}) (interface{}, error) {
	// 1 arg: name of variable. return cleaned string value
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if s, ok := args[0].(string); ok {
		_, val, err := fl.GetValue(s, nil)
		if err != nil {
			panic(err)
		}

		out := []byte{}
		for _, c := range val {
			if c == 0 {
				break
			}
			out = append(out, c)
		}

		return string(out), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

func (fl *FileLayout) evalBitSet(args ...interface{}) (interface{}, error) {
	// 2 args: 1) field name 2) bit
	// returns bool true if bit is set

	if len(args) != 2 {
		return nil, fmt.Errorf("expected exactly 2 argument")
	}

	v2, ok := args[1].(int)
	if !ok {
		return nil, fmt.Errorf("2nd arg: expected int, got %T", args[1])
	}

	if v2 > 7 {
		return nil, fmt.Errorf("TODO: bitset() over bit 7")
	}

	if s, ok := args[0].(string); ok {
		_, val, err := fl.GetValue(s, nil)
		if err != nil {
			panic(err)
		}

		res := (val[0])&(1<<(v2)) != 0
		return res, nil
	}

	return nil, fmt.Errorf("1st arg: expected string, got %T", args[0])
}

func (fl *FileLayout) evalEither(args ...interface{}) (interface{}, error) {
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

func (fl *FileLayout) evalNot(args ...interface{}) (interface{}, error) {
	// 2-n arg: reference value, list of values. returns true if 1st number is not any of the others
	if len(args) < 2 {
		return nil, fmt.Errorf("expected at least 2 arguments")
	}
	ref := 0
	found := false
	for i, j := range args {
		if v, ok := j.(int); ok {
			if i == 0 {
				ref = v
			} else {
				if v == ref {
					found = true
				}
			}
		} else {
			return false, fmt.Errorf("expected int, got %T", args[0])
		}
	}
	return !found, nil
}

var (
	evalVariables = make(map[string]interface{})
)

func (fl *FileLayout) evaluateExpr(in string, df *value.DataField) (interface{}, error) {
	in = strings.ReplaceAll(in, "OFFSET", fmt.Sprintf("%d", fl.offset))
	in = strings.ReplaceAll(in, "self.", df.Label+".")

	// fast path: if "in" looks like decimal number just convert it
	if v, err := strconv.Atoi(in); err == nil {
		return uint64(v), nil
	}

	eval := goval.NewEvaluator()

	for idx, layout := range fl.Structs {
		if layout.evaluated {
			if idx+2 < len(fl.Structs) {
				// must not skip the struct currently being parsed when evaluateExpr() is invoked
				log.Debug().Msgf("Skipping %s while evaluating %s. idx %d, len %d",
					layout.Name, df.Label, idx+1, len(fl.Structs))

				continue
			}
		}
		mapped := make(map[string]interface{})
		for _, field := range layout.Fields {
			mapped[field.Format.Label] = fl.GetFieldValue(&field)
		}

		mapped["index"] = int(layout.Index)
		evalVariables[layout.Name] = mapped

		for _, child := range layout.Children {
			mapped = make(map[string]interface{})

			for _, field := range child.Fields {
				log.Debug().Msgf("Adding child node %s to %s", field.Format.Label, layout.Name)
				mapped[field.Format.Label] = fl.GetFieldValue(&field)
			}

			mapped["index"] = int(child.Index)
			evalVariables[child.Name] = mapped
		}

		fl.Structs[idx].evaluated = true
	}

	for _, constant := range fl.DS.Constants {
		if len(constant.Value) <= 4 || len(constant.Value) == 8 {
			evalVariables[constant.Name] = int(value.AsUint64Raw(constant.Value))
		}
	}
	evalVariables["FILE_SIZE"] = int(fl.size)

	log.Debug().Str("in", in).Str("block", df.Label).Msgf("EVALUATING at %06x", fl.offset)

	functions := make(map[string]goval.ExpressionFunction)
	functions["peek_i32"] = fl.evalPeekI32
	functions["peek_i16"] = fl.evalPeekI16
	functions["peek_i8"] = fl.evalPeekI8
	functions["atoi"] = fl.evalAtoi
	functions["otoi"] = fl.evalOtoi
	functions["ceil"] = fl.evalCeil
	functions["abs"] = fl.evalAbs
	functions["alignment"] = fl.evalAlignment
	functions["offset"] = fl.evalOffset
	functions["len"] = fl.evalLen
	functions["not"] = fl.evalNot
	functions["either"] = fl.evalEither
	functions["sevenbitstring"] = fl.evalSevenBitString
	functions["bitset"] = fl.evalBitSet
	functions["cleanstring"] = fl.evalCleanString
	result, err := eval.Evaluate(in, evalVariables, functions)
	if err != nil {
		if DEBUG_EVAL {
			spew.Dump(evalVariables)
		}
		return 0, EvaluateError{input: in, msg: err.Error()}
	}

	return result, nil
}
