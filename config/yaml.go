package config

import (
	"fmt"
	"io"
	"regexp"

	"go.yaml.in/yaml/v3"
)

var yamlLocation = regexp.MustCompile(`line [0-9]+`)
var yamlUnknownField = regexp.MustCompile(`field ([A-Za-z_][A-Za-z0-9_]*) not found`)

// DecodeYAML reads one strict document. Diagnostics deliberately omit scalar
// values because configuration may contain sensitive data.
func DecodeYAML(reader io.Reader, value any) error {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	if err := decoder.Decode(value); err != nil {
		if err == io.EOF {
			return io.EOF
		}
		location := yamlLocation.FindString(err.Error())
		if field := yamlUnknownField.FindStringSubmatch(err.Error()); len(field) > 1 {
			return fmt.Errorf("invalid configuration at %s: unknown field %q", location, field[1])
		}
		return fmt.Errorf("invalid YAML configuration at %s (syntax, duplicate key, or value type)", location)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("configuration must contain exactly one YAML document")
	}
	return nil
}
