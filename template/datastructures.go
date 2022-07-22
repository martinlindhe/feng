package template

import (
	"fmt"
	"log"

	"github.com/martinlindhe/feng/value"
	"gopkg.in/yaml.v2"
)

// structure of a evaluated ./templates/ yaml file. see Template for the raw structure corresponding to the yaml file
type DataStructure struct {

	// constants derived from eq & bit matches
	Constants []Constant

	// evaluated file structs
	EvaluatedStructs []evaluatedStruct

	Layout []value.DataField

	Magic []Magic

	// default endian
	Endian string

	// if template lacks magic bytes
	NoMagic bool

	// extensions
	Extensions []string

	// lastpath/filename-without-ext, eg "archives/zip"
	BaseName string
}

func UnmarshalTemplateIntoDataStructure(b []byte, basename string) (*DataStructure, error) {
	var template Template
	err := yaml.Unmarshal([]byte(b), &template)
	if err != nil {
		return nil, err
	}

	ds, err := NewDataStructureFrom(&template, basename)
	if err != nil {
		return nil, err
	}

	if len(ds.Layout) == 0 {
		return nil, fmt.Errorf("no layout section found in template")
	}

	return ds, err
}

func NewDataStructureFrom(template *Template, basename string) (*DataStructure, error) {
	if DEBUG_PATTERNS {
		log.Println("NewDataStructureFrom", basename)
	}
	constants, err := template.evaluateConstants()
	if err != nil {
		return nil, err
	}
	structs, err := template.evaluateStructs()
	if err != nil {
		return nil, err
	}
	layout, err := template.evaluateLayout()
	if err != nil {
		return nil, err
	}

	return &DataStructure{
		Constants:        constants,
		EvaluatedStructs: structs,
		Layout:           layout,
		Magic:            template.Magic,
		NoMagic:          template.NoMagic,
		Endian:           template.Endian,
		Extensions:       template.Extensions,
		BaseName:         basename,
	}, nil
}

// looks up layout name from sections
func (ds *DataStructure) FindStructure(name string) (*evaluatedStruct, error) {
	for _, str := range ds.EvaluatedStructs {
		if name == str.Name {
			return &str, nil
		}
	}
	return nil, fmt.Errorf("not found in structs: '%s'", name)
}
