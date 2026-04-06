package main

import (
	"context"
	"log"

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

It first creates an array of size one more than the messages array
It then appends the messages to the this new array after converting them to gemini api compatible syntax
at last it appends the user query to the array and we are done
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

func run(ctx context.Context, key string, query string, model string) string {
	// initializing the database
	db := initDB()
	defer db.Close()

	// by default last 20 messages are sent as context
	messages := getHistory(db, 20)

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  key,
	})
	if err != nil {
		log.Fatal(err)
	}

	var tools = []*genai.Tool{
		{
			GoogleSearch: &genai.GoogleSearch{},
		},
	}

	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{
		Tools:          tools,
		ThinkingConfig: &genai.ThinkingConfig{
			//      ThinkingLevel: genai.Ptr[string]("HIGH"),
		},
	}

	contents := historyToGenAIContents(messages, query)

	result, err := client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		log.Fatal(err)
	}
	return result.Text()
}
