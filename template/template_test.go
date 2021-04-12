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
		if err != nil {
			return err
		}
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

		if len(tpl.Extensions) == 0 {
			return fmt.Errorf("extensions missing")
		}

		_, err = UnmarshalTemplateIntoDataStructure(templateData)
		assert.Equal(t, nil, err)

		return nil
	})
	assert.Equal(t, nil, err)
}

func TestEvaluateTemplateConstants(t *testing.T) {
	templateData := `
constants:
  u8[2] I: c'I' 00
  u8[3] X: c'XX' 00
layout:
  -
`
	ds, err := UnmarshalTemplateIntoDataStructure([]byte(templateData))
	assert.Equal(t, nil, err)

	assert.Equal(t, EvaluatedConstant{value.DataField{Kind: "u8", Range: "2", Label: "I"}, []byte{0x49, 0x0}}, ds.Constants[0])
	assert.Equal(t, EvaluatedConstant{value.DataField{Kind: "u8", Range: "3", Label: "X"}, []byte{0x58, 0x58, 0x0}}, ds.Constants[1])
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
    u8 Unit:
      bit b0001: B0
      bit b0110: Rest
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

	assert.Equal(t, &DataStructure{
		Constants: []EvaluatedConstant{},
		structs: []evaluatedStruct{
			{Name: "header", Expressions: []Expression{
				{Field: value.DataField{Kind: "u8", Range: "2", Slice: false, Label: "Signature"}, Pattern: value.DataPattern{Known: true, Pattern: []uint8{0xff, 0xd8}, Value: ""}, Children: []Expression{}, MatchPatterns: []MatchPattern{}}},
			},
			{Name: "segment", Expressions: []Expression{
				{Field: value.DataField{Kind: "u8", Range: "2", Slice: false, Label: "Signature"}, Pattern: value.DataPattern{Known: false, Pattern: []uint8(nil), Value: ""}, Children: []Expression{}, MatchPatterns: []MatchPattern{}},
				{Field: value.DataField{Kind: "u16", Range: "", Slice: false, Label: "Unit"}, Pattern: value.DataPattern{Known: false, Pattern: []uint8(nil), Value: ""}, Children: []Expression{}, MatchPatterns: []MatchPattern{
					{Operation: "eq", Pattern: "00", Label: "No units"},
					{Operation: "eq", Pattern: "01", Label: "Pixels per inch"},
					{Operation: "default", Pattern: "", Label: "invalid"},
				}},
				{Field: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Unit"}, Pattern: value.DataPattern{Known: false, Pattern: []uint8(nil), Value: ""}, Children: []Expression{}, MatchPatterns: []MatchPattern{
					{Operation: "bit", Pattern: "b0001", Label: "B0"},
					{Operation: "bit", Pattern: "b0110", Label: "Rest"},
					{Operation: "default", Pattern: "", Label: "invalid"},
				}},
				{Field: value.DataField{Kind: "if", Range: "", Slice: false, Label: "Unit == 4"}, Pattern: value.DataPattern{Known: false, Pattern: []uint8(nil), Value: ""}, Children: []Expression{
					{Field: value.DataField{Kind: "u8", Range: "", Slice: false, Label: "Child data"}, Pattern: value.DataPattern{Known: false, Pattern: []uint8(nil), Value: ""}, Children: []Expression{}, MatchPatterns: []MatchPattern{}},
				}, MatchPatterns: []MatchPattern{}},
			}},
		},
		Layout: []value.DataField{
			{Kind: "header", Range: "", Slice: false, Label: "Header"},
			{Kind: "segment", Range: "header.Unit", Slice: false, Label: "segments"},
			{Kind: "other_segment", Range: "", Slice: true, Label: "other_segments"},
		},
	}, ds)

}
