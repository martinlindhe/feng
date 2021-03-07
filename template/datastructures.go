package template

import (
	"fmt"

	"github.com/martinlindhe/feng/value"
	"gopkg.in/yaml.v2"
)

type DataStructure struct {

	// evaluated constants
	constants []evaluatedConstant

	// evaluated file structs
	structs []evaluatedStruct

	Layout []value.DataField
}

func UnmarshalTemplateIntoDataStructure(b []byte) (*DataStructure, error) {
	var template Template
	err := yaml.Unmarshal([]byte(b), &template)
	if err != nil {
		return nil, err
	}

	ds, err := NewDataStructureFrom(&template)
	if err != nil {
		return nil, err
	}

	if len(ds.Layout) == 0 {
		return nil, fmt.Errorf("no layout section found in template")
	}

	return ds, err
}

func NewDataStructureFrom(template *Template) (*DataStructure, error) {
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
	return &DataStructure{constants, structs, layout}, nil
}

// looks up layout name from sections
func (ds *DataStructure) FindStructure(df *value.DataField) (*evaluatedStruct, error) {
	for _, str := range ds.structs {
		if df.Kind == str.Name {
			return &str, nil
		}
	}
	return nil, fmt.Errorf("not found in structs: '%s'", df.Kind)
}
