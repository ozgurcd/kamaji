package target

import (
	"bytes"
	"encoding/json"
	"fmt"
	"kamaji/obj"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	cfg "kamaji/config"
	"kamaji/tools"
)

// RuleSchema describes accepted options without executing the rule.
type RuleSchema struct {
	Execution    *RuleExecution `yaml:"execution,omitempty" json:"execution,omitempty"`
	Language     string         `yaml:"language" json:"language,omitempty"`
	Description  string         `yaml:"description" json:"description,omitempty"`
	AllowUnknown *bool          `yaml:"allow_unknown" json:"allow_unknown,omitempty"`
	Variables    map[string]any `yaml:"-" json:"variables"`
}

// RuleExecution specifies a language-independent process invocation.
type RuleExecution struct {
	Mode    string   `yaml:"mode" json:"mode"`
	Command []string `yaml:"command,omitempty" json:"command,omitempty"`
}

func readSchema(path string) (RuleSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RuleSchema{}, err
	}
	var raw struct {
		Execution    *RuleExecution `yaml:"execution"`
		Language     string         `yaml:"language"`
		Description  string         `yaml:"description"`
		AllowUnknown *bool          `yaml:"allow_unknown"`
		Variables    map[any]any    `yaml:"variables"`
	}
	if err := cfg.DecodeYAML(bytes.NewReader(data), &raw); err != nil {
		return RuleSchema{}, err
	}
	if raw.Variables == nil {
		return RuleSchema{}, fmt.Errorf("variables section must be a map")
	}
	switch raw.Language {
	case "", "python":
		raw.Language = "python"
	case "go", "golang":
		raw.Language = "go"
	}
	if raw.Execution == nil {
		if raw.Language != "python" && raw.Language != "go" {
			return RuleSchema{}, fmt.Errorf("rule language requires execution settings; select interpreter or executable mode")
		}
	} else {
		switch raw.Execution.Mode {
		case "interpreter":
			if len(raw.Execution.Command) == 0 || strings.TrimSpace(raw.Execution.Command[0]) == "" {
				return RuleSchema{}, fmt.Errorf("interpreter execution requires a nonempty command argument list")
			}
		case "executable":
			if len(raw.Execution.Command) != 0 {
				return RuleSchema{}, fmt.Errorf("executable execution uses the target rule path and cannot specify command")
			}
		default:
			return RuleSchema{}, fmt.Errorf("execution mode must be interpreter or executable")
		}
		for _, argument := range raw.Execution.Command {
			if strings.ContainsRune(argument, '\x00') {
				return RuleSchema{}, fmt.Errorf("execution command arguments cannot contain NUL")
			}
		}
	}
	schema := RuleSchema{Execution: raw.Execution, Language: raw.Language, Description: raw.Description, AllowUnknown: raw.AllowUnknown, Variables: map[string]any{}}
	for key, value := range raw.Variables {
		name, ok := key.(string)
		if !ok || name == "" {
			return RuleSchema{}, fmt.Errorf("variable names must be nonempty strings")
		}
		schema.Variables[name] = value
	}
	return schema, nil
}

func (scope *Manager) Schema() (RuleSchema, error) {
	return scope.SchemaFor(scope.Runtime.Config.ExecTarget)
}

// SchemaFor reads the definition next to a specific rule without changing runtime state.
func (scope *Manager) SchemaFor(selected obj.ExecTarget) (RuleSchema, error) {
	rule, err := (&tools.Context{Runtime: scope.Runtime}).GetRule(selected)
	if err != nil {
		return RuleSchema{}, err
	}
	if !filepath.IsAbs(rule) {
		rule = filepath.Join(scope.Runtime.Config.WorkspaceConfig.RulesDir, rule)
	}
	return readSchema(filepath.Join(filepath.Dir(rule), "rule_definition.yaml"))
}

type variableDefinition struct {
	kind             string
	required         bool
	fallback         any
	hasDefault       bool
	enum             []any
	minimum, maximum *big.Rat
}

func numeric(value any) (*big.Rat, bool) {
	if value == nil {
		return nil, false
	}
	k := reflect.TypeOf(value).Kind()
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		n, ok := new(big.Rat).SetString(fmt.Sprint(value))
		return n, ok
	}
	return nil, false
}

func matchesType(value any, kind string) bool {
	if value == nil {
		return false
	}
	k := reflect.TypeOf(value).Kind()
	switch kind {
	case "string":
		return k == reflect.String
	case "bool":
		return k == reflect.Bool
	case "int":
		return k >= reflect.Int && k <= reflect.Int64 || k >= reflect.Uint && k <= reflect.Uint64
	case "number":
		_, ok := numeric(value)
		return ok
	case "map":
		return k == reflect.Map
	case "list":
		return k == reflect.Slice || k == reflect.Array
	}
	return false
}

func parseDefinition(raw any) (variableDefinition, error) {
	d := variableDefinition{}
	if kind, ok := raw.(string); ok {
		d.kind = kind
	} else {
		var fields map[string]any
		switch v := raw.(type) {
		case map[string]any:
			fields = v
		case map[any]any:
			fields = tools.NormalizeMap(v)
		default:
			return d, fmt.Errorf("definition must be a type or mapping")
		}
		for key := range fields {
			switch key {
			case "type", "description", "mandatory", "default", "enum", "minimum", "maximum":
			default:
				return d, fmt.Errorf("unknown schema field %q", key)
			}
		}
		d.kind, _ = fields["type"].(string)
		if v, ok := fields["description"]; ok {
			if _, ok := v.(string); !ok {
				return d, fmt.Errorf("description must be a string")
			}
		}
		if v, ok := fields["mandatory"]; ok {
			var valid bool
			d.required, valid = v.(bool)
			if !valid {
				return d, fmt.Errorf("mandatory must be boolean")
			}
		}
		d.fallback, d.hasDefault = fields["default"]
		if v, ok := fields["enum"]; ok {
			var valid bool
			d.enum, valid = v.([]any)
			if !valid || len(d.enum) == 0 {
				return d, fmt.Errorf("enum must be a nonempty list")
			}
		}
		for _, key := range []string{"minimum", "maximum"} {
			if v, exists := fields[key]; exists {
				n, ok := numeric(v)
				if !ok || (d.kind != "int" && d.kind != "number") {
					return d, fmt.Errorf("numeric bounds require a numeric type and value")
				}
				if key == "minimum" {
					d.minimum = n
				} else {
					d.maximum = n
				}
			}
		}
	}
	switch d.kind {
	case "string", "bool", "int", "number", "map", "list":
	default:
		return d, fmt.Errorf("invalid type")
	}
	if d.minimum != nil && d.maximum != nil && d.minimum.Cmp(d.maximum) > 0 {
		return d, fmt.Errorf("minimum exceeds maximum")
	}
	for _, v := range d.enum {
		if !matchesType(v, d.kind) {
			return d, fmt.Errorf("enum member has incorrect type")
		}
	}
	if d.hasDefault {
		if err := d.validate(d.fallback); err != nil {
			return d, fmt.Errorf("invalid default: %w", err)
		}
	}
	return d, nil
}

func (d variableDefinition) validate(value any) error {
	if !matchesType(value, d.kind) {
		return fmt.Errorf("expected %s", d.kind)
	}
	normalized := tools.NormalizeMap(map[any]any{"value": value})["value"]
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("value is not JSON serializable")
	}
	if len(d.enum) > 0 {
		found := false
		for _, candidate := range d.enum {
			if a, ok := numeric(value); ok {
				if b, ok := numeric(candidate); ok && a.Cmp(b) == 0 {
					found = true
					break
				}
			}
			b, err := json.Marshal(tools.NormalizeMap(map[any]any{"value": candidate})["value"])
			if err == nil && bytes.Equal(encoded, b) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value is not an allowed enum member")
		}
	}
	if n, ok := numeric(value); ok {
		if d.minimum != nil && n.Cmp(d.minimum) < 0 {
			return fmt.Errorf("value is below minimum")
		}
		if d.maximum != nil && n.Cmp(d.maximum) > 0 {
			return fmt.Errorf("value exceeds maximum")
		}
	}
	return nil
}

func validateSchema(schema RuleSchema, values map[string]any) error {
	if schema.AllowUnknown != nil && !*schema.AllowUnknown {
		names := make([]string, 0, len(values))
		for name := range values {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, ok := schema.Variables[name]; !ok {
				return fmt.Errorf("unknown option %q", name)
			}
		}
	}
	names := make([]string, 0, len(schema.Variables))
	for name := range schema.Variables {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		d, err := parseDefinition(schema.Variables[name])
		if err != nil {
			return fmt.Errorf("invalid definition for variable %q: %w", name, err)
		}
		value, exists := values[name]
		if !exists && d.hasDefault {
			if values == nil {
				return fmt.Errorf("cannot apply defaults to nil configuration")
			}
			value = d.fallback
			values[name] = value
			exists = true
		}
		if !exists {
			if d.required {
				return fmt.Errorf("mandatory variable %q is missing", name)
			}
			continue
		}
		if err := d.validate(value); err != nil {
			return fmt.Errorf("variable %q: %w", name, err)
		}
	}
	return nil
}
