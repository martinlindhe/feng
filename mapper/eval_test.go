package mapper

import (
	"testing"

	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func mockFile(t *testing.T, filename string, data []byte) afero.File {
	appFS := afero.NewMemMapFs()
	err := afero.WriteFile(appFS, filename, data, 0644)
	if err != nil {
		panic(err)
	}
	f, err := appFS.Open(filename)
	assert.Nil(t, err)
	return f
}

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

	f := mockFile(t, "in", []byte{
		// Header
		0x06, // Val1
		0x03, // Len1
	})

	fl, err := MapReader(&MapReaderConfig{
		F:  f,
		DS: ds,
	})
	assert.Equal(t, nil, err)

	test := []struct {
		expr     string
		expected int64
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

	f := mockFile(t, "in", []byte{
		// Header
		'H', 'i', // Name
		0x01, // Val
	})

	fl, err := MapReader(&MapReaderConfig{
		F:  f,
		DS: ds,
	})
	assert.Equal(t, nil, err)

	test := []struct {
		expr     string
		expected string
	}{
		{"Header.Name", "Hi"},
		{`Header.Name + " there"`, "Hi there"},
		{`"Hello " + Header.Name`, "Hello Hi"},
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

	f := mockFile(t, "in", []byte{
		// Header
		0x02, 0x01, // CRC
		0x08, 0x00, // HeaderSize
		0xf0, 0xf1, 0xf2, 0xf3, // Reserved
	})

	fl, err := MapReader(&MapReaderConfig{
		F:  f,
		DS: ds,
	})
	assert.Equal(t, nil, err)

	// Header
	_, val, err := fl.GetValue("Header.Reserved", &ds.Layout[0])
	assert.Equal(t, nil, err)
	assert.Equal(t, 4, len(val), val)
}

func TestEvalAlignment(t *testing.T) {
	var alignmentTests = []struct {
		dataLen         int
		alignment       int
		expectedPadding int
	}{
		{2, 4, 2},
		{3, 4, 1},
		{4, 4, 0},
		{5, 4, 3},
		{6, 4, 2},
		{26, 4, 2},
		{0x6800, 0x800, 0},
		{0x67FF, 0x800, 1},
		{0x6001, 0x800, 0x7FF},
	}

	for _, tt := range alignmentTests {
		i, err := evalAlignment(tt.dataLen, tt.alignment)
		assert.Equal(t, nil, err)
		assert.Equal(t, tt.expectedPadding, i)
	}
}
