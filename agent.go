package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

const (
	maxAgentToolRounds = 20
)

func buildAgentGenerationConfig(reasoning string) *genai.GenerateContentConfig {
	cfg := buildGenerationConfig(reasoning)

	includeServerSideToolInvocations := true
	cfg.ToolConfig = &genai.ToolConfig{
		IncludeServerSideToolInvocations: &includeServerSideToolInvocations,
	}

	shellCommandSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to run using `bash -lc`.",
			},
			"working_directory": map[string]any{
				"type":        "string",
				"description": "Optional working directory. Relative paths are resolved from the current directory.",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"description": "Optional timeout between 1 and 180 seconds.",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Why this command is needed.",
			},
		},
		"required": []string{"command"},
	}

	memoryIDSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"memory_id": map[string]any{
				"type":        "string",
				"description": "The stable memory ID returned by memory_view.",
			},
		},
		"required": []string{"memory_id"},
	}

	memoryContentSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "string",
				"description": "Memory text content.",
			},
		},
		"required": []string{"content"},
	}

	memoryUpdateSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"memory_id": map[string]any{
				"type":        "string",
				"description": "The stable memory ID returned by memory_view.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "New memory text content.",
			},
		},
		"required": []string{"memory_id", "content"},
	}

	readFileSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative file path to read.",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Optional: only return content from this line number onwards (1-indexed).",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Optional: only return content up to this line number (inclusive).",
			},
		},
		"required": []string{"path"},
	}

	writeFileSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative file path to edit.",
			},
			"old_str": map[string]any{
				"type":        "string",
				"description": "The exact string to find in the file. Must match exactly including whitespace and newlines.",
			},
			"new_str": map[string]any{
				"type":        "string",
				"description": "The string to replace it with. Can be empty string to delete old_str.",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Optional: explanation of what this edit does.",
			},
		},
		"required": []string{"path", "old_str", "new_str"},
	}

	clipboardSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Either 'read' to get clipboard content or 'write' to set clipboard content.",
				"enum":        []string{"read", "write"},
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Text content to write to clipboard. Required when action is 'write', ignored when 'read'.",
			},
		},
		"required": []string{"action"},
	}

	listsSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action to perform: create_list, delete_list, get_lists, add_item, update_item, delete_item, get_items",
				"enum":        []string{"create_list", "delete_list", "get_lists", "add_item", "update_item", "delete_item", "get_items"},
			},
			"list_name": map[string]any{
				"type":        "string",
				"description": "Name of the list to operate on",
			},
			"item_id": map[string]any{
				"type":        "integer",
				"description": "ID of the item to update or delete",
			},
			"item_content": map[string]any{
				"type":        "string",
				"description": "Text content of the item",
			},
			"status": map[string]any{
				"type":        "string",
				"description": "Item status: 'pending' or 'done'",
				"enum":        []string{"pending", "done"},
			},
		},
		"required": []string{"action"},
	}

	httpRequestSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "Complete URL including scheme (e.g., https://api.example.com/users)",
			},
			"method": map[string]any{
				"type":        "string",
				"description": "HTTP method to use",
				"enum":        []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "Optional HTTP headers as key-value pairs (e.g., {\"Authorization\": \"Bearer token\"})",
			},
			"body": map[string]any{
				"type":        "string",
				"description": "Request body as a string. Must be pre-serialized JSON if needed. Ignored for GET and DELETE methods.",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"description": "Request timeout between 1 and 60 seconds. Default is 10.",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Optional explanation for why this request is being made",
			},
		},
		"required": []string{"url"},
	}

	sendDocumentsSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filepath": map[string]any{
				"type":        "string",
				"description": "Complete file path of the file that needs to be sent over Telegram",
			},
		},
	}

	sendImagesSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filepath": map[string]any{
				"type":        "string",
				"description": "Complete file path of the image file that needs to be sent over Telegram",
			},
		},
	}

	cfg.Tools = append(cfg.Tools, &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:                 "run_shell_command",
				Description:          "Run a shell command on the local machine and return stdout/stderr/exit code.",
				ParametersJsonSchema: shellCommandSchema,
			},
			{
				Name:                 "memory_view",
				Description:          "List all currently stored memories with their IDs.",
				ParametersJsonSchema: map[string]any{"type": "object", "properties": map[string]any{}},
			},
			{
				Name:                 "memory_add",
				Description:          "Add a new memory to long-term memory storage.",
				ParametersJsonSchema: memoryContentSchema,
			},
			{
				Name:                 "memory_delete",
				Description:          "Delete one memory by memory_id.",
				ParametersJsonSchema: memoryIDSchema,
			},
			{
				Name:                 "memory_update",
				Description:          "Update one memory by memory_id.",
				ParametersJsonSchema: memoryUpdateSchema,
			},
			{
				Name:                 "read_file",
				Description:          "Read a file from disk and return its contents with line numbers. Supports reading specific line ranges.",
				ParametersJsonSchema: readFileSchema,
			},
			{
				Name:                 "write_file",
				Description:          "Perform a partial edit on an existing file using exact string replacement. Finds old_str and replaces it with new_str. Requires user approval unless --yolo is active.",
				ParametersJsonSchema: writeFileSchema,
			},
			{
				Name:                 "clipboard",
				Description:          "Read from or write to the system clipboard. Write operations require user approval unless --yolo is active.",
				ParametersJsonSchema: clipboardSchema,
			},
			{
				Name:                 "lists",
				Description:          "Manage named lists with items that have status tracking (pending/done). Supports creating lists, adding items, updating item status, and querying lists and items.",
				ParametersJsonSchema: listsSchema,
			},
			{
				Name:                 "http_request",
				Description:          "Make HTTP requests to any URL and receive structured responses. Supports GET, POST, PUT, PATCH, and DELETE methods with custom headers and body. POST/PUT/PATCH/DELETE require user approval unless --yolo is active.",
				ParametersJsonSchema: httpRequestSchema,
			},
			{
				Name:                 "send_document_over_telegram",
				Description:          "Send a document to the user when they are communicating over Telegram.",
				ParametersJsonSchema: sendDocumentsSchema,
			},
			{
				Name:                 "send_image_over_telegram",
				Description:          "Send an image to the user when they are communicating over Telegram.",
				ParametersJsonSchema: sendImagesSchema,
			},
		},
	})

	return cfg
}

func runAgentTurn(ctx context.Context, db *sql.DB, key string, query string, model string, reasoning string, autoApprove bool) string {
	messages := getHistory(db, 20)
	// since we have crud tools for managing memories, model can interact with them directly and injecting memory into the prompt will only clutter it
	//	queryWithMemory := injectMemoryContext(ctx, query)
	contents := historyToGenAIContents(messages, query)

	client := newGeminiClient(ctx, key)
	config := buildAgentGenerationConfig(reasoning)

	for range maxAgentToolRounds {
		result, err := client.Models.GenerateContent(ctx, model, contents, config)
		if err != nil {
			return fmt.Sprintf("Agent request failed: %v", err)
		}

		if len(result.Candidates) > 0 && result.Candidates[0] != nil && result.Candidates[0].Content != nil {
			logThoughts(result.Candidates[0].Content.Parts)
		}

		functionCalls := result.FunctionCalls()
		if len(functionCalls) == 0 {
			return strings.TrimSpace(result.Text())
		}

		if len(result.Candidates) > 0 && result.Candidates[0] != nil && result.Candidates[0].Content != nil {
			contents = append(contents, result.Candidates[0].Content)
		}

		responses := make([]*genai.Part, 0, len(functionCalls))
		for _, call := range functionCalls {
			response := handleAgentFunctionCall(call, autoApprove, db)
			responses = append(responses, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:       call.ID,
					Name:     call.Name,
					Response: response,
				},
			})
		}

		contents = append(contents, &genai.Content{
			Role:  string(genai.RoleUser),
			Parts: responses,
		})
	}

	return "Agent stopped after too many tool iterations. Try a more specific instruction."
}

func handleAgentFunctionCall(call *genai.FunctionCall, autoApprove bool, db *sql.DB) map[string]any {
	if call == nil {
		return map[string]any{"error": map[string]any{"message": "nil function call"}}
	}

	switch call.Name {
	case "run_shell_command":
		req, err := parseShellCommandRequest(call.Args)
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}

		printToolCall(req)
		if !autoApprove && !askForCommandApproval() {
			printToolDenied()
			return map[string]any{
				"error": map[string]any{"message": "command denied by user"},
				"output": map[string]any{
					"command":           req.Command,
					"working_directory": req.WorkingDirectory,
					"timeout_seconds":   req.TimeoutSeconds,
				},
			}
		}

		res := executeShellCommand(req)
		printToolResult(res)
		return res.toToolResponse()
	case "memory_view":
		records, err := listStoredMemoryRecords()
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}
		items := make([]map[string]any, 0, len(records))
		for _, record := range records {
			items = append(items, map[string]any{
				"id":      record.ID,
				"content": record.Content,
			})
		}
		return map[string]any{
			"count":    len(items),
			"memories": items,
		}
	case "memory_add":
		content, err := requiredStringArg(call.Args, "content")
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}
		record, err := addMemory(context.Background(), content)
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}
		return map[string]any{
			"ok": true,
			"memory": map[string]any{
				"id":      record.ID,
				"content": record.Content,
			},
		}
	case "memory_delete":
		id, err := requiredStringArg(call.Args, "memory_id")
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}
		if err := deleteMemoryByID(context.Background(), id); err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}
		return map[string]any{
			"ok":        true,
			"memory_id": id,
		}
	case "memory_update":
		id, err := requiredStringArg(call.Args, "memory_id")
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}
		content, err := requiredStringArg(call.Args, "content")
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}
		record, err := updateMemoryByID(context.Background(), id, content)
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}
		return map[string]any{
			"ok": true,
			"memory": map[string]any{
				"id":      record.ID,
				"content": record.Content,
			},
		}
	case "read_file":
		req, err := parseReadFileRequest(call.Args)
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}
		res := executeReadFile(req)
		return res.toToolResponse()
	case "write_file":
		req, err := parseWriteFileRequest(call.Args)
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}

		printWriteFileCall(req)
		if !autoApprove && !askForEditApproval() {
			printEditDenied()
			return map[string]any{
				"error": map[string]any{"message": "edit denied by user"},
				"output": map[string]any{
					"path":   req.Path,
					"reason": req.Reason,
				},
			}
		}

		res := executeWriteFile(req)
		printWriteFileResult(res)
		return res.toToolResponse()
	case "send_document_over_telegram":
		fmt.Println("[DEBUG] Agent function: send_document_over_telegram")
		req, err := parseDocumentSendRequest(call.Args)
		if err != nil {
			fmt.Println("[ERROR] Failed to parse document send request:", err)
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}

		fmt.Println("[DEBUG] Sending document to Telegram:", req.FilePath)
		sendError := sendDocument(req.FilePath)
		if sendError != nil {
			fmt.Println("[ERROR] Document send failed:", sendError)
			return map[string]any{"error": map[string]any{"message": sendError.Error()}}
		}

		fmt.Println("[DEBUG] Document handler completed successfully")
		return map[string]any{
			"ok":       true,
			"filepath": req.FilePath,
		}
	case "send_image_over_telegram":
		fmt.Println("[DEBUG] Agent function: send_image_over_telegram")
		req, err := parseImageSendRequest(call.Args)
		if err != nil {
			fmt.Println("[ERROR] Failed to parse image send request:", err)
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}

		fmt.Println("[DEBUG] Sending image to Telegram:", req.FilePath)
		sendError := sendImage(req.FilePath)
		if sendError != nil {
			fmt.Println("[ERROR] Image send failed:", sendError)
			return map[string]any{"error": map[string]any{"message": sendError.Error()}}
		}

		fmt.Println("[DEBUG] Image handler completed successfully")
		return map[string]any{
			"ok":       true,
			"filepath": req.FilePath,
		}

	case "clipboard":
		req, err := parseClipboardRequest(call.Args)
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}

		if req.Action == "write" {
			printClipboardWriteCall(req)
			if !autoApprove && !askForClipboardApproval() {
				printClipboardDenied()
				return map[string]any{
					"error": map[string]any{"message": "clipboard write denied by user"},
					"output": map[string]any{
						"action": req.Action,
					},
				}
			}
		}

		res := executeClipboard(req)
		printClipboardResult(res)
		return res.toToolResponse()
	case "lists":
		req, err := parseListsRequest(call.Args)
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}

		res := executeLists(db, req, autoApprove)
		return res.toToolResponse()
	case "http_request":
		req, err := parseHTTPRequestRequest(call.Args)
		if err != nil {
			return map[string]any{"error": map[string]any{"message": err.Error()}}
		}

		// Require approval for POST, PUT, PATCH, DELETE unless --yolo is active
		if !autoApprove && (req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" || req.Method == "DELETE") {
			printHTTPRequestCall(req)
			if !askForHTTPRequestApproval() {
				printHTTPRequestDenied()
				return map[string]any{
					"error": map[string]any{"message": "request denied by user"},
					"output": map[string]any{
						"method": req.Method,
						"url":    req.URL,
					},
				}
			}
		}

		res := executeHTTPRequest(req)
		printHTTPRequestResult(res)
		return res.toToolResponse()
	default:
		return map[string]any{
			"error": map[string]any{
				"message": "unsupported function call",
				"name":    call.Name,
			},
		}
	}
}

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
