package main

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/philippgille/chromem-go"
)

type memoryRecord struct {
	ID      string
	Content string
}

func listStoredMemories() ([]string, error) {
	records, err := listStoredMemoryRecords()
	if err != nil {
		return nil, err
	}

	memories := make([]string, 0, len(records))
	for _, record := range records {
		memories = append(memories, record.Content)
	}
	return memories, nil
}

func listStoredMemoryRecords() ([]memoryRecord, error) {
	collectionPath, err := collectionDirPath("user-memory")
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(collectionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []memoryRecord{}, nil
		}
		return nil, fmt.Errorf("failed to read memory collection directory: %w", err)
	}

	records := make([]memoryRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Name() == "00000000" {
			continue
		}

		docPath := filepath.Join(collectionPath, entry.Name())
		file, err := os.Open(docPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to open memory file %q: %w", docPath, err)
		}

		var doc chromem.Document
		decodeErr := gob.NewDecoder(file).Decode(&doc)
		closeErr := file.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode memory file %q: %w", docPath, decodeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("failed to close memory file %q: %w", docPath, closeErr)
		}

		if strings.TrimSpace(doc.Content) == "" || strings.TrimSpace(doc.ID) == "" {
			continue
		}

		records = append(records, memoryRecord{ID: doc.ID, Content: doc.Content})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Content < records[j].Content
	})
	return records, nil
}

func addMemory(ctx context.Context, content string) (memoryRecord, error) {
	if memoryCollection == nil {
		return memoryRecord{}, fmt.Errorf("memory collection is not initialized")
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return memoryRecord{}, fmt.Errorf("memory content cannot be empty")
	}

	doc := chromem.Document{
		ID:      hash(content),
		Content: content,
	}
	if err := storeDocuments(ctx, memoryCollection, []chromem.Document{doc}); err != nil {
		return memoryRecord{}, err
	}

	return memoryRecord{ID: doc.ID, Content: doc.Content}, nil
}

func deleteMemoryByID(ctx context.Context, id string) error {
	if memoryCollection == nil {
		return fmt.Errorf("memory collection is not initialized")
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("memory ID cannot be empty")
	}

	if err := memoryCollection.Delete(ctx, nil, nil, id); err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}
	return nil
}

func updateMemoryByID(ctx context.Context, id string, newContent string) (memoryRecord, error) {
	if memoryCollection == nil {
		return memoryRecord{}, fmt.Errorf("memory collection is not initialized")
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return memoryRecord{}, fmt.Errorf("memory ID cannot be empty")
	}

	newContent = strings.TrimSpace(newContent)
	if newContent == "" {
		return memoryRecord{}, fmt.Errorf("updated memory content cannot be empty")
	}

	doc := chromem.Document{ID: id, Content: newContent}
	if err := memoryCollection.AddDocument(ctx, doc); err != nil {
		return memoryRecord{}, fmt.Errorf("failed to update memory: %w", err)
	}

	return memoryRecord{ID: id, Content: newContent}, nil
}

func collectionDirPath(collectionName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error while fetching the home directory of the user: %w", err)
	}
	dbPath := filepath.Join(homeDir, "db")
	return filepath.Join(dbPath, shortHashHex(collectionName)), nil
}

func shortHashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:4])
}
