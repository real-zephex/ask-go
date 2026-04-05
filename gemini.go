package main

import (
	"context"
	"log"
	"google.golang.org/genai"
)

func run(ctx context.Context, key string, query string, model string) string {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey: key,
	})
	if err != nil {
		log.Fatal(err)
	}

  var tools = []*genai.Tool{
    {
      GoogleSearch: &genai.GoogleSearch{
      },
    },
  }

  var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{
    Tools: tools,
    ThinkingConfig: &genai.ThinkingConfig{
//      ThinkingLevel: genai.Ptr[string]("HIGH"),
    },
  }

  var contents = []*genai.Content{
    {
      Role: "user",
      Parts: []*genai.Part{
        {
          Text: query,
        },
      },
    },
  }

	result, err := client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		log.Fatal(err)
	}
	return result.Text();
}
