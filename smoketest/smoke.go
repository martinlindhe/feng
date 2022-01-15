package smoketest

import (
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type SmoketestsYaml struct {
	Root  string `yaml:"root"`
	Input []struct {
		Folder string   `yaml:"folder"`
		Files  []string `yaml:"files"`
	} `yaml:"input"`
}

type InOutPair struct {
	In  string
	Out string
}

func (sty *SmoketestsYaml) GenerateFilenames() (res []InOutPair) {

	for _, smoke := range sty.Input {

		for _, file := range smoke.Files {

			filename := filepath.Join(sty.Root, smoke.Folder, file)
			outfile := filepath.Join("./reference", smoke.Folder, file+".out")
			pair := InOutPair{
				In:  filename,
				Out: outfile,
			}
			res = append(res, pair)
		}
	}
	return
}

func UnmarshalData(in []byte) (*SmoketestsYaml, error) {
	var smoketests SmoketestsYaml
	err := yaml.Unmarshal(in, &smoketests)
	return &smoketests, err
}
