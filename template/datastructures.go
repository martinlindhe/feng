package template

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"

	"github.com/martinlindhe/feng/value"
)

// structure of a evaluated ./templates/ yaml file. see Template for the raw structure corresponding to the yaml file
type DataStructure struct {

	// constants derived from eq & bit pattern matches
	Constants []Constant

	// evaluated file structs
	EvaluatedStructs []EvaluatedStruct

	Layout []value.DataField

	Magic []Magic

	// default endian
	Endian string

	// if template lacks magic bytes
	NoMagic bool

	// extensions, used to match if no_magic is true
	Extensions []string

	// used to match if no_magic is true
	Filenames []string

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
		log.Print("NewDataStructureFrom", basename)
	}

	constants, err := template.evaluateConstants()
	if err != nil {
		log.Warn().Err(err).Msgf("%s: evaluateConstants failed", basename)
		return nil, err
	}

	structs, err := template.evaluateStructs()
	if err != nil {
		log.Warn().Err(err).Msgf("%s: evaluateStructs failed", basename)
		return nil, err
	}

	layout, err := template.evaluateLayout()
	if err != nil {
		log.Warn().Err(err).Msgf("%s: evaluateLayout failed", basename)
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
		Filenames:        template.Filenames,
		BaseName:         basename,
	}, nil
}

// looks up layout name from sections
func (ds *DataStructure) FindStructure(name string) (*EvaluatedStruct, error) {
	for _, str := range ds.EvaluatedStructs {
		if name == str.Name {
			return &str, nil
		}
	}
	return nil, fmt.Errorf("not found in structs: '%s'", name)
}
