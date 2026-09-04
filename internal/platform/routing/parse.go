package routing

import (
	"bytes"
	"io"

	"gopkg.in/yaml.v3"
)

// ParseYAMLDocuments decodes every YAML document contained in data.
//
// Route files may hold several route-group documents separated by "---".
// A single yaml.Unmarshal call would silently return only the first
// document, so every consumer of route files must decode document by
// document. Documents that decode to an empty struct (stray separators)
// are returned as zero values; callers should skip them (e.g. by checking
// Metadata.Name for route configs).
func ParseYAMLDocuments[T any](data []byte) ([]T, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var docs []T
	for {
		var doc T
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
