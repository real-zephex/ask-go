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
       
       ## Primary Rule
       The USER message is the source of truth. Extract facts primarily from what the user says, does, or implies.
       The ASSISTANT message is supporting context only — use it to better understand the user's intent, but only extract from it if it reveals something the user confirmed, acted on, or clearly agrees with.
       Never store facts that exist only in the assistant's response with no signal from the user.
       
       ## What to extract (from user message)
       - Stable preferences: tone, formatting, response style, workflow habits
       - Long-term goals and ongoing projects
       - Hard constraints: things the user explicitly wants or refuses
       - Durable personal context: location, role, tech stack, tools, environment
       - Corrections or updates to previously known facts (prefix with "UPDATE:")
       
       ## What to extract (from assistant message only if)
       - The assistant states a fact about the user that the user did not contradict or correct
       - The assistant infers something the user implicitly confirmed through follow-up
       - The assistant names a tool, project, or detail the user clearly accepted as accurate
       
       ## What to never extract
       - One-off or throwaway requests
       - Temporary details with no future relevance
       - Generic facts not specific to this user
       - Raw outputs, logs, code snippets, error messages
       - Sensitive data: passwords, API keys, tokens, credentials
       - Facts stated only by the assistant with no user confirmation signal
       - Questions the user asked without revealing personal context
       
       ## Output rules
       - Return ONLY a valid JSON array of strings
       - No markdown, no explanation, no wrapping
       - Each string: concise, plain English, third-person ("User prefers...")
       - If a fact updates a previously known one: "UPDATE: User now uses X instead of Y"
       - If nothing is worth storing: []
       
       ## Examples
       
       User: "can you rewrite this in a more concise way, I hate verbose responses"
       Assistant: "Sure, here's the rewritten version..."
       → ["User prefers concise, non-verbose responses."]
       
       User: "what is 142 * 37"
       Assistant: "The answer is 5254."
       → []
       
       User: "yeah that looks right, also I moved to Neovim full time now"
       Assistant: "Got it, I'll keep that in mind. Neovim is great for your workflow."
       → ["UPDATE: User now uses Neovim full time as their primary editor."]
       
       User: "run the build and check for errors"
       Assistant: "You're on Fedora Linux with Go 1.25 installed."
       → []
       (assistant stated a fact, user did not confirm — do not store)
       
       User: "yeah exactly, I'm on Fedora"
       Assistant: "Got it."
       → ["User is on Fedora Linux."]
       (user explicitly confirmed — store it)
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
