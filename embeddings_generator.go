package main

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

func generateEmbeddings(ctx context.Context, key string, message string) ([]float32, error) {
	client := newGeminiClient(ctx, key)

	contents := []*genai.Content{
		genai.NewContentFromText(message, genai.RoleUser),
	}
	result, err := client.Models.EmbedContent(ctx, "gemini-embedding-2", contents, nil)
	if err != nil {
		fError := fmt.Errorf("An error occure while trying to embed messages using Gemini: %v", err)
		return []float32{}, fError
	}

	actualEmbeddings := *result.Embeddings[0]
	return actualEmbeddings.Values, nil
}

func chromemCustomGenerator(ctx context.Context, text string) ([]float32, error) {
	e, _ := checkForEnv()
	embeddings, err := generateEmbeddings(ctx, e, text)
	if err != nil {
		return nil, err
	}

	return embeddings, nil
}
