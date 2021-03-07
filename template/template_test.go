package template

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/martinlindhe/feng/value"
)

// evaluate all templates, validate some fields
func TestWalkTemplates(t *testing.T) {
	searchDir := "../templates/"
	err := filepath.Walk(searchDir, func(path string, fi os.FileInfo, err error) error {
		if fi == nil {
			t.Fatalf("invalid path " + searchDir)
		}
		if fi.IsDir() {
			return nil
		}

		if filepath.Ext(fi.Name()) != ".yml" {
			return nil
		}

		templateData, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		fmt.Println("processing", path)

		var tpl Template
		err = yaml.Unmarshal(templateData, &tpl)
		if err != nil {
			return err
		}
		switch tpl.Kind {
		case "image", "archive":
		default:
			return fmt.Errorf("unknown kind '%s", tpl.Kind)
		}

		return nil
	})
	assert.Equal(t, nil, err)
}

func TestEvaluateTemplateConstants(t *testing.T) {
	templateData := `
constants:
  u8[2] I: c'I' 00
  u8[3] X: c'XX' 00
`
	ds, err := UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	assert.Equal(t, evaluatedConstant{value.DataField{Kind: "u8", Range: "2", Label: "I"}, []byte{0x49, 0x0}}, ds.constants[0])
	assert.Equal(t, evaluatedConstant{value.DataField{Kind: "u8", Range: "3", Label: "X"}, []byte{0x58, 0x58, 0x0}}, ds.constants[1])
}

func TestEvaluateStructsAndLayout(t *testing.T) {
	templateData := `
structs:
  header:
    u8[2] Signature: ff d8
  segment:
    u8[2] Signature: ??
    u16 Unit:
      eq 00: No units
      eq 01: Pixels per inch
      default: invalid
    if Unit == 4:
      u8 Child data: ??

layout:
  - header Header
  - segment[header.Unit] segments
  - other_segment[] other_segments
`
	ds, err := UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	// structs
	assert.Equal(t, 2, len(ds.structs))
	assert.Equal(t, "header", ds.structs[0].Name)
	assert.Equal(t, true, ds.structs[0].Expressions[0].Pattern.Known)
	assert.Equal(t, []byte{0xff, 0xd8}, ds.structs[0].Expressions[0].Pattern.Pattern)
	assert.Equal(t, "segment", ds.structs[1].Name)

	assert.Equal(t, false, ds.structs[1].Expressions[1].Pattern.Known)
	assert.Equal(t, "u16", ds.structs[1].Expressions[1].Field.Kind)
	assert.Equal(t, "Unit", ds.structs[1].Expressions[1].Field.Label)

	// pattern match
	assert.Equal(t, []matchPattern{
		{operation: "eq", pattern: "00", label: "No units"},
		{operation: "eq", pattern: "01", label: "Pixels per inch"},
		{operation: "default", pattern: "", label: "invalid"}},
		ds.structs[1].Expressions[1].matchPatterns)

	assert.Equal(t, "if", ds.structs[1].Expressions[2].Field.Kind)
	assert.Equal(t, "Unit == 4", ds.structs[1].Expressions[2].Field.Label)
	assert.Equal(t, "u8", ds.structs[1].Expressions[2].children[0].Field.Kind)
	assert.Equal(t, "Child data", ds.structs[1].Expressions[2].children[0].Field.Label)

	// layouts
	assert.Equal(t, "segment", ds.Layout[1].Kind)
	assert.Equal(t, "header.Unit", ds.Layout[1].Range)
	assert.Equal(t, "segments", ds.Layout[1].Label)
}
