package tools

import "fmt"

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringProperty(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func requiredString(input map[string]any, key string) (string, error) {
	value, ok := input[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string", key)
	}
	if text == "" && key != "new_string" {
		return "", fmt.Errorf("parameter %s must not be empty", key)
	}
	return text, nil
}

func optionalString(input map[string]any, key string) (string, error) {
	value, ok := input[key]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string", key)
	}
	return text, nil
}
