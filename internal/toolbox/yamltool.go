package toolbox

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// ValidateYAML parses YAML and reports whether it is well-formed.
func ValidateYAML(input string) error {
	var v any
	return yaml.Unmarshal([]byte(input), &v)
}

// FormatYAML re-serialises YAML with consistent 2-space indentation.
func FormatYAML(input string) (string, error) {
	var v any
	if err := yaml.Unmarshal([]byte(input), &v); err != nil {
		return "", err
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// YAMLToJSON converts a YAML document to pretty-printed JSON.
func YAMLToJSON(input string) (string, error) {
	var v any
	if err := yaml.Unmarshal([]byte(input), &v); err != nil {
		return "", err
	}
	out, err := json.MarshalIndent(normaliseYAML(v), "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// JSONToYAML converts a JSON document to YAML.
func JSONToYAML(input string) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return "", err
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// normaliseYAML converts map[any]any (produced by the YAML decoder for nested
// maps) into map[string]any so encoding/json can serialise it.
func normaliseYAML(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = normaliseYAML(val)
		}
		return t
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[toStr(k)] = normaliseYAML(val)
		}
		return m
	case []any:
		for i, val := range t {
			t[i] = normaliseYAML(val)
		}
		return t
	default:
		return v
	}
}

func toStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
