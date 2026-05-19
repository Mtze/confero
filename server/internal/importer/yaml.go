package importer

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type yamlDocument struct {
	Conferences []ConferenceInput `yaml:"conferences"`
}

// YAMLImporter parses a YAML document containing a `conferences` key.
type YAMLImporter struct{}

// Parse implements Importer.
func (y *YAMLImporter) Parse(r io.Reader) ([]ConferenceInput, error) {
	var doc yamlDocument
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	return doc.Conferences, nil
}
