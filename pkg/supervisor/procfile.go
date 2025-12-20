package supervisor

import (
	"os"
	"regexp"

	orderedmap "github.com/gnuos/omap"
	"go.yaml.in/yaml/v3"
)

type ProcfileConfig struct {
	*orderedmap.OrderedMap[string, string]
}

func (p *ProcfileConfig) IsValid() bool {
	re := regexp.MustCompile(`^[A-Za-z]+[A-Za-z0-9-_]+$`)
	for pair := p.Oldest(); pair != nil; pair = pair.Next() {
		if !re.MatchString(pair.Key) {
			return false
		}
	}
	return true
}

func LoadProcfile(name string) (*ProcfileConfig, error) {
	omap := orderedmap.New[string, string]()
	pfile := &ProcfileConfig{omap}

	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, omap)
	if err != nil {
		return nil, err
	}

	data = nil

	return pfile, nil
}
