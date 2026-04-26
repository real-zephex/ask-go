package main

import (
	"context"
	"database/sql"
	"log"
	"strings"

	"google.golang.org/genai"
)

type GeminiMessage struct {
	Role string
	Text string
}

// takes those gemini message and converts them to gemini api compatible format
func (m GeminiMessage) ToGenAIContent() *genai.Content {
	return &genai.Content{
		Role: m.Role,
		Parts: []*genai.Part{
			{Text: m.Text},
		},
	}
}

// takes message from the db and converts them to gemini message struct
func messageFromDB(m Message) GeminiMessage {
	return GeminiMessage{
		Role: m.Role,
		Text: m.Content,
	}
}

/*
This function takes in the messages array from the database and user query and returns gemini api compatible format
- It first creates an array of size one more than the messages array
- It then appends the messages to the this new array after converting them to gemini api compatible syntax
- at last it appends the user query to the array and we are done

make(type, currentLength, fullLength)
*/
func historyToGenAIContents(messages []Message, query string) []*genai.Content {
	contents := make([]*genai.Content, 0, len(messages)+1)

	// DB rows are fetched newest first, but Gemini context should be oldest first.
	for i := len(messages) - 1; i >= 0; i-- {
		contents = append(contents, messageFromDB(messages[i]).ToGenAIContent())
	}

	contents = append(contents, GeminiMessage{
		Role: "user",
		Text: query,
	}.ToGenAIContent())

	return contents
}

func newGeminiClient(ctx context.Context, key string) *genai.Client {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  key,
	})
	if err != nil {
		log.Fatal(err)
	}
	return client
}

// used to build the configuration for gemini like tools, thinking level, system prompts stuff and stuff
func buildGenerationConfig(reasoning string) *genai.GenerateContentConfig {
	var tools = []*genai.Tool{
		{
			GoogleSearch: &genai.GoogleSearch{},
		},
	}

	config := &genai.GenerateContentConfig{
		Tools: tools,
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingLevel:   genai.ThinkingLevel(reasoning),
			IncludeThoughts: true,
		},
	}

	if strings.TrimSpace(runtimeSystemPrompt) != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: runtimeSystemPrompt}},
		}
	}

	return config
}

// takes in the message and logs the thoughts
func logThoughts(parts []*genai.Part) {
	var thoughts strings.Builder
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.Text != "" && part.Thought {
			thoughts.WriteString(part.Text)
		}
	}

	if thoughts.Len() > 0 {
		render("# Thoughts\n" + thoughts.String() + "---")
	}
}

// the OG function, this is used when stream is set to off. implemented this function myself
func run(ctx context.Context, db *sql.DB, key string, query string, model string, reasoning string) string {
	// by default last 20 messages are sent as context
	messages := getHistory(db, 20)
	queryWithMemory := injectMemoryContext(ctx, query)

	client := newGeminiClient(ctx, key)
	config := buildGenerationConfig(reasoning)
	contents := historyToGenAIContents(messages, queryWithMemory)

	result, err := client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		log.Fatal(err)
	}

	if len(result.Candidates) > 0 && result.Candidates[0] != nil && result.Candidates[0].Content != nil {
		logThoughts(result.Candidates[0].Content.Parts)
	}

	return result.Text()
}

// AI overlords hired some workers to make this function. I get how it works!
func runStream(
	ctx context.Context,
	db *sql.DB,
	key string,
	query string,
	model string,
	reasoning string,
	onTextChunk func(string),
	onComplete func(string),
) string {
	messages := getHistory(db, 20)
	queryWithMemory := injectMemoryContext(ctx, query)

	client := newGeminiClient(ctx, key)
	config := buildGenerationConfig(reasoning)
	contents := historyToGenAIContents(messages, queryWithMemory)

	var answer strings.Builder
	var thoughts strings.Builder

	for chunk, err := range client.Models.GenerateContentStream(ctx, model, contents, config) {
		if err != nil {
			log.Fatal(err)
		}

		text := chunk.Text()
		if text != "" {
			answer.WriteString(text)
			if onTextChunk != nil {
				onTextChunk(text)
			}
		}

		for _, candidate := range chunk.Candidates {
			if candidate == nil || candidate.Content == nil {
				continue
			}
			for _, part := range candidate.Content.Parts {
				if part == nil {
					continue
				}
				if part.Text != "" && part.Thought {
					thoughts.WriteString(part.Text)
				}
			}
		}
	}

	finalAnswer := answer.String()
	if onComplete != nil {
		onComplete(finalAnswer)
	}

	if thoughts.Len() > 0 {
		render("# Thoughts\n" + thoughts.String() + "---")
	}

	return finalAnswer
}
