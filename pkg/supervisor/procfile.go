package supervisor

import (
	"os"
	"regexp"

	"go.yaml.in/yaml/v3"
)

type ProcfileConfig map[string]string

func (p *ProcfileConfig) IsValid() bool {
	re := regexp.MustCompile(`^[A-Za-z]+[A-Za-z0-9-_]+$`)
	for k := range *p {
		if !re.MatchString(k) {
			return false
		}
	}
	return true
}

func LoadProcfile(name string) (*ProcfileConfig, error) {
	var pfile = &ProcfileConfig{}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, pfile)
	if err != nil {
		return nil, err
	}

	return pfile, nil
}
