package mapper

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
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

	//log.Info().Msgf("eval string %s, %v", in, fl.scriptFunctions)

	result, err := fl.evaluateExpr(in, df)
	if err != nil {
		return "", err
	}

	switch v := result.(type) {
	case string:
		if DEBUG_EVAL {
			log.Info().Msgf("EvaluateStringExpression: %s => %s", in, v)
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
			log.Info().Msgf("EvaluateExpression: %s => %d", in, v)
		}
		return int64(v), nil

	case uint64:
		if DEBUG_EVAL {
			log.Info().Msgf("EvaluateExpression: %s => %d", in, v)
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

// 1 arg: hex string
func (fl *FileLayout) evalPeekI32(args ...interface{}) (interface{}, error) {
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
			log.Info().Msgf("peek_i32 AT OFFSET %06x: %04x", offset, int(val))
		}
		return int(val), nil
	}
	if offset, ok := args[0].(int); ok {
		if int64(offset) >= fl.size {
			return 0, fmt.Errorf("out of range %06x", offset)
		}
		val, _ := fl.peekU32(int64(offset))

		if DEBUG_EVAL {
			log.Info().Msgf("peek_i32 AT OFFSET %06x: %04x", offset, val)
		}
		return int(val), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

// 1 arg: hex string
func (fl *FileLayout) evalPeekI16(args ...interface{}) (interface{}, error) {
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
			log.Info().Msgf("peek_i16 AT OFFSET %06x: %04x", offset, val)
		}
		return int(val), nil
	}
	if offset, ok := args[0].(int); ok {
		if int64(offset) >= fl.size {
			return 0, fmt.Errorf("out of range %06x", offset)
		}
		val, _ := fl.peekU16(int64(offset))

		if DEBUG_EVAL {
			log.Info().Msgf("peek_i16 AT OFFSET %06x: %04x", offset, val)
		}
		return int(val), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

// 1 arg: hex string
func (fl *FileLayout) evalPeekI8(args ...interface{}) (interface{}, error) {
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
			log.Info().Msgf("peek_i8 AT OFFSET %06x: %04x", offset, val)
		}
		return int(val), nil
	}
	if offset, ok := args[0].(int); ok {
		if int64(offset) >= fl.size {
			return 0, fmt.Errorf("out of range %06x", offset)
		}
		val, _ := fl.peekU8(int64(offset))
		if DEBUG_EVAL {
			log.Info().Msgf("peek_i8 AT OFFSET %06x: %02x", offset, val)
		}
		return int(val), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

// convert alphanumeric string to int
// 1 arg: string. return integer value
func (fl *FileLayout) evalAtoi(args ...interface{}) (interface{}, error) {
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

// convert octal numeric string to int
// 1 arg: string. return integer value
func (fl *FileLayout) evalOtoi(args ...interface{}) (interface{}, error) {
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

// 1 arg: int. return integer value
// returns ceil of float64
func (fl *FileLayout) evalCeil(args ...interface{}) (interface{}, error) {
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

// 2 args: 1) size value, 2) alignment. return the alignment needed for size value to fit into "alignment"
// example: value 4, align 4. returns 0
// example: value 4, align 8. returns 4
func evalAlignment(args ...interface{}) (interface{}, error) {
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
		log.Info().Msgf("aligned %d, %d => %d", v1, v2, i)
	}
	return i, nil
}

// 1 arg: name of variable. return its offset as int
func (fl *FileLayout) evalOffset(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if s, ok := args[0].(string); ok {
		i, err := fl.GetOffset(s, nil)
		if err != nil {
			panic(err)
		}
		if DEBUG_EVAL {
			log.Info().Msgf("eval offset('%s') => %06x", s, i)
		}
		return i, nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

// 1 arg: name of variable. return its data length as int
func (fl *FileLayout) evalLen(args ...interface{}) (interface{}, error) {
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

// 1 arg: name of variable. return manipulated string value
func (fl *FileLayout) evalSevenBitString(args ...interface{}) (interface{}, error) {
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

// 1 arg: name of variable. return cleaned string value
func (fl *FileLayout) evalCleanString(args ...interface{}) (interface{}, error) {
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

// 1 arg: name of variable. return extension from filename (".ext")
func (fl *FileLayout) evalExt(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if s, ok := args[0].(string); ok {
		return filepath.Ext(s), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

// 1 arg: name of variable. return filename without extension
func (fl *FileLayout) evalNoExt(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if s, ok := args[0].(string); ok {
		return strings.TrimSuffix(s, filepath.Ext(s)), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

// 2 arg: name of variable. return filename without path
func (fl *FileLayout) evalBasename(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("expected exactly 1 argument")
	}
	if s, ok := args[0].(string); ok {
		return filepath.Base(s), nil
	}
	return nil, fmt.Errorf("expected string, got %T", args[0])
}

// fetch a value from a field in a struct array
// 3 args: 1) struct index 2) struct name 3) field name
func (fl *FileLayout) evalStruct(args ...interface{}) (interface{}, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("expected exactly 3 argument")
	}

	v1, ok := args[0].(int)
	if !ok {
		return nil, fmt.Errorf("1st arg: expected int, got %T", args[1])
	}
	v2, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("2nd arg: expected string, got %T", args[1])
	}
	v3, ok := args[2].(string)
	if !ok {
		return nil, fmt.Errorf("3rd arg: expected string, got %T", args[1])
	}

	st, err := fl.findStruct(fmt.Sprintf("%s_%d", v2, v1)) // File_3
	if err != nil {
		log.Warn().Err(err).Msgf("field %s[%d].%s not found", v2, v1, v3)
		return "", nil
	}

	field, err := st.findField(v3)
	if err != nil {
		return nil, err
	}

	return fl.GetFieldValue(field), nil
}

func (st *Struct) findField(name string) (*Field, error) {
	for _, field := range st.Fields {
		if field.Format.Label == name {
			return &field, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

// returns the struct from the parsed layout by name
func (fl *FileLayout) findStruct(name string) (*Struct, error) {
	for _, layout := range fl.Structs {
		if layout.Name == name {
			return layout, nil
		}
		for _, child := range layout.Children {
			if child.Name == name {
				return child, nil
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

// 2 args: 1) field name 2) bit
// returns bool true if bit is set
func (fl *FileLayout) evalBitSet(args ...interface{}) (interface{}, error) {
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

// 2-n arg: reference value, list of values. returns true if 1st number is in the others
func (fl *FileLayout) evalEither(args ...interface{}) (interface{}, error) {
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

// 2-n arg: reference value, list of values. returns true if 1st number is not any of the others
func (fl *FileLayout) evalNot(args ...interface{}) (interface{}, error) {
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

func (fl *FileLayout) createInitialConstants() {
	if fl.scriptVariables != nil {
		return
	}

	fl.scriptVariables = make(map[string]interface{})

	if fl.DS != nil {
		for _, constant := range fl.DS.Constants {
			if len(constant.Value) <= 4 || len(constant.Value) == 8 {
				fl.scriptVariables[constant.Name] = int(value.AsUint64Raw(constant.Value))
			}
		}
	}

	fl.scriptVariables["FILE_SIZE"] = int(fl.size)
	fl.scriptVariables["FILE_NAME"] = fl._f.Name()
}

func (fl *FileLayout) evaluateExpr(in string, df *value.DataField) (interface{}, error) {
	in = strings.ReplaceAll(in, "OFFSET", fmt.Sprintf("%d", fl.offset))
	in = strings.ReplaceAll(in, "self.", df.Label+".")

	fl.createInitialConstants()

	// fast path: if "in" looks like decimal number just convert it
	if v, err := strconv.Atoi(in); err == nil {
		return uint64(v), nil
	}

	log.Debug().Str("in", in).Str("block", df.Label).Msgf("EVALUATING at %06x", fl.offset)

	started := time.Now()

	for idx, layout := range fl.Structs {
		if layout.evaluated {
			if idx+1 < len(fl.Structs) {
				// HACK WORKAROUND, due to to evaluateExpr() being called recursively  TODO FIX by avoid recurse
				// must not skip the struct currently being parsed when evaluateExpr() is invoked
				log.Debug().Msgf("Skipping %s while evaluating %s. idx %d, len %d",
					layout.Name, df.Label, idx+1, len(fl.Structs))
				continue
			}
		}
		if DEBUG_EVAL {
			log.Warn().Msgf("Processing struct %d: %s", idx, layout.Name)
		}
		if fl.scriptVariables[layout.Name] == nil {
			fl.scriptVariables[layout.Name] = make(map[string]interface{})
		}
		for _, field := range layout.Fields {
			if DEBUG_EVAL {
				log.Warn().Msgf("adding %s.%s", layout.Name, field.Format.Label)
			}
			fl.scriptVariables[layout.Name].(map[string]interface{})[field.Format.Label] = fl.GetFieldValue(&field)
		}

		fl.scriptVariables[layout.Name].(map[string]interface{})["index"] = int(layout.Index)

		for _, child := range layout.Children {
			if fl.scriptVariables[child.Name] == nil {
				fl.scriptVariables[child.Name] = make(map[string]interface{})
			}

			for _, field := range child.Fields {
				log.Warn().Msgf("Adding child node %s to %s", field.Format.Label, layout.Name)
				fl.scriptVariables[child.Name].(map[string]interface{})[field.Format.Label] = fl.GetFieldValue(&field)
			}

			fl.scriptVariables[child.Name].(map[string]interface{})["index"] = int(child.Index)
			log.Warn().Msgf("adding %s.%s", df.Label, child.Name)
		}

		fl.Structs[idx].evaluated = true
	}

	//spew.Dump(fl.scriptVariables)
	fl.evaluatedExpressions++
	result, err := fl.eval.Evaluate(in, fl.scriptVariables, fl.scriptFunctions)
	fl.evaluatedExpressionTime += time.Since(started)

	if err != nil {
		if DEBUG_EVAL {
			spew.Dump(fl.scriptVariables)
		}
		return 0, EvaluateError{input: in, msg: err.Error()}
	}

	return result, nil
}
