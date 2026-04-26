package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/philippgille/chromem-go"
	"google.golang.org/genai"
)

var schema = &genai.Schema{
	Type:     genai.TypeArray,
	MaxItems: genai.Ptr[int64](10),
	Items: &genai.Schema{
		Type:      genai.TypeString,
		MinLength: genai.Ptr[int64](1),
	},
}

func runRemember(ctx context.Context, userQuery string, aiResponse string) ([]string, error) {
	key, _ := checkForEnv()
	client := newGeminiClient(ctx, key)

	prompt := `
		You are a memory extraction engine for a personal AI assistant.
	
		Task:
		Given a conversation turn, extract only information that is useful for long-term personalization and future interactions. 

		What is worth remembering:
		- Stable user preferences (style, tone, formatting, workflow)
		- Long-term goals and ongoing projects
		- Important constraints and boundaries (things user wants/doesn’t want)
		- Recurring habits or routines
		- Durable personal context that improves future responses   
	
		What is NOT worth remembering:   
		- One-off requests   
		- Temporary details unlikely to matter later   
		- Generic facts not specific to this user   
		- Raw logs, long outputs, or noisy text   
		- Secrets or sensitive data (passwords, API keys, tokens, private credentials)   

		Output rules:   
		- Return ONLY a JSON array of strings.   
		- No markdown, no explanation, no extra keys.   
		- Each string must be a concise memory statement in plain English.   
		- Do not include duplicates or near-duplicates.   
		- Normalize phrasing so memories are reusable and clear.   
		- If nothing is worth storing, return [].   

		Examples:   
		Input meaning: user says they prefer concise answers and are building a Go CLI agent.   
		Output:   ["User prefers concise answers.", "User is building a Go CLI agent project."]   

		Input meaning: user asks a one-time math question.   
		Output:[]

		The messages are passed as chat history below.
	`

	content := []*genai.Content{
		genai.NewContentFromText(userQuery, genai.RoleUser),
		genai.NewContentFromText(aiResponse, genai.RoleModel),
	}

	config := &genai.GenerateContentConfig{
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: schema,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: prompt}},
		},
	}

	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash-lite",
		content,
		config,
	)
	if err != nil {
		fError := fmt.Errorf("There was en error communicating with the Gemini API for managing memories: %v", err)
		return []string{}, fError
	}

	var memories []string
	if err := json.Unmarshal([]byte(result.Text()), &memories); err != nil {
		return []string{}, fmt.Errorf("failed to parse memory output: %w", err)
	}

	clean := make([]string, 0, len(memories))
	seen := map[string]struct{}{}
	for _, memory := range memories {
		m := strings.TrimSpace(memory)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		clean = append(clean, m)
	}

	return clean, nil
}

func rememberTurn(ctx context.Context, userQuery string, aiResponse string) (int, error) {
	if memoryCollection == nil {
		return 0, nil
	}

	memories, err := runRemember(ctx, userQuery, aiResponse)
	if err != nil {
		return 0, err
	}
	if len(memories) == 0 {
		return 0, nil
	}

	docs := make([]chromem.Document, 0, len(memories))
	for _, memory := range memories {
		docs = append(docs, chromem.Document{
			ID:      hash(memory),
			Content: memory,
		})
	}

	if err := storeDocuments(ctx, memoryCollection, docs); err != nil {
		return 0, err
	}

	return len(docs), nil
}
