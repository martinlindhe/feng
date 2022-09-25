package mapper

import (
	"bytes"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/maja42/goval"
	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateExpression(t *testing.T) {
	templateData := `
structs:
  header:
    u8 Val1: ??
    u8 Len1: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	data := []byte{
		// Header
		0x06, // Val1
		0x03, // Len1
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	test := []struct {
		expr     string
		expected uint64
	}{
		{"4+2", 6},
		{"2 << 10", 0x800},
		{"abs(-44)", 44},
		{"Header.Val1 * 2", 12},
	}
	for _, h := range test {
		a, err := fl.EvaluateExpression(h.expr, &value.DataField{})
		assert.Nil(t, err)
		assert.Equal(t, h.expected, a)
	}
}

func TestEvaluateStringExpression(t *testing.T) {
	templateData := `
structs:
  header:
    ascii[2] Name: ??
    u8 Val: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	data := []byte{
		// Header
		'H', 'i', // Name
		0x01, // Val
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	test := []struct {
		expr     string
		expected string
	}{
		{"Header.Name", "Hi"},
	}
	for _, h := range test {
		a, err := fl.EvaluateStringExpression(h.expr, &value.DataField{})
		assert.Nil(t, err)
		assert.Equal(t, h.expected, a)
	}
}

func TestCalcArraySize(t *testing.T) {
	// based on calc used in archives/rar.yml
	templateData := `
endian: little
structs:
  header:
    u16 CRC: ??
    u16 HeaderSize: ??
    u8[self.HeaderSize - (OFFSET - offset("self.CRC"))] Reserved: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	data := []byte{
		// Header
		0x02, 0x01, // CRC
		0x08, 0x00, // HeaderSize
		0xf0, 0xf1, 0xf2, 0xf3, // Reserved
	}

	fl, err := MapReader(bytes.NewReader(data), ds)
	assert.Equal(t, nil, err)

	// Header
	_, val, err := fl.GetValue("Header.Reserved", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, 4, len(val), val)
}

func TestGovalStrings(t *testing.T) {

	eval := goval.NewEvaluator()

	variables := make(map[string]interface{})
	functions := make(map[string]goval.ExpressionFunction)

	result, err := eval.Evaluate(` "hello" `, variables, functions)

	assert.Nil(t, err)

	spew.Dump(result)
}
