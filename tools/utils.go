package tools

import (
	"errors"
	"fmt"
	"strings"
)

func requiredStringArg(args map[string]any, key string) (string, error) {
	if args == nil {
		return "", errors.New("function args missing")
	}
	raw, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument: %s", key)
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("argument '%s' must be a non-empty string", key)
	}
	return strings.TrimSpace(value), nil
}