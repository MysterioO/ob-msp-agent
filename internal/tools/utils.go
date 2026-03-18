package tools

import "fmt"

func requireString(args map[string]any, key string) (string, error) {
	val, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument: %s", key)
	}
	strVal, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("argument %s must be a string", key)
	}
	return strVal, nil
}
