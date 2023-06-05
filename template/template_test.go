package template

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/value"
)

// evaluate all templates, validate some fields
func TestEvaluateAllTemplates(t *testing.T) {

	fs.WalkDir(feng.Templates, ".", func(path string, d fs.DirEntry, err2 error) error {
		// cannot happen
		if err2 != nil {
			panic(err2)
		}
		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".yml" {
			return nil
		}

		templateData, err := fs.ReadFile(feng.Templates, path)
		assert.Equal(t, nil, err)

		fmt.Println("processing", path)

		var tpl Template
		err = yaml.Unmarshal(templateData, &tpl)
		assert.Equal(t, nil, err)

		switch tpl.Kind {
		case "image", "archive", "system", "executable", "document", "debug", "media", "network", "video", "audio", "font", "bytecode", "game", "asset", "container", "":
		default:
			t.Errorf("unknown kind '%s' in template %s", tpl.Kind, tpl.Name)
			t.FailNow()
		}

		_, err = UnmarshalTemplateIntoDataStructure(templateData, path)
		assert.Equal(t, nil, err)
		return nil
	})
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
	ds, err := UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	assert.Equal(t, &DataStructure{
		Constants: []Constant{
			{Name: "No units", Value: []byte{0x00}},
			{Name: "Pixels per inch", Value: []byte{0x01}},
			{Name: "B0", Value: []byte{0x01}},
			{Name: "Rest", Value: []byte{0x06}},
		},
		EvaluatedStructs: []EvaluatedStruct{
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
