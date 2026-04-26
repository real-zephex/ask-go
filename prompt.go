package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

var runtimeSystemPrompt string

func loadSystemPromptFromFile(path string) string {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		log.Fatalf("failed to read system prompt file %q: %v", cleanPath, err)
	}

	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		log.Fatalf("system prompt file %q is empty", cleanPath)
	}

	return prompt
}
