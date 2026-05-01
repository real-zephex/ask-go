package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/genai"
)

var (
	clipboardDaemonMutex sync.Mutex
	clipboardDaemonCmd   *exec.Cmd
)

const (
	maxAgentToolRounds      = 20
	defaultShellTimeout     = 30
	minimumShellTimeout     = 1
	maximumShellTimeout     = 180
	maxShellOutputLength    = 8000
	maxFileOutputLength     = 8000
	maxClipboardOutputLength = 8000
)

type shellCommandRequest struct {
	Command          string
	WorkingDirectory string
	TimeoutSeconds   int
	Reason           string
}

type clipboardRequest struct {
	Action  string
	Content string
}

type readFileRequest struct {
	Path      string
	StartLine int
	EndLine   int
}

type writeFileRequest struct {
	Path   string
	OldStr string
	NewStr string
	Reason string
}

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

func parseShellCommandRequest(args map[string]any) (shellCommandRequest, error) {
	if args == nil {
		return shellCommandRequest{}, errors.New("function args missing")
	}

	cmdValue, ok := args["command"]
	if !ok {
		return shellCommandRequest{}, errors.New("missing required argument: command")
	}

	command, ok := cmdValue.(string)
	if !ok || strings.TrimSpace(command) == "" {
		return shellCommandRequest{}, errors.New("argument 'command' must be a non-empty string")
	}

	wd, err := resolveWorkingDirectory(args["working_directory"])
	if err != nil {
		return shellCommandRequest{}, err
	}

	timeoutSeconds := defaultShellTimeout
	if rawTimeout, ok := args["timeout_seconds"]; ok {
		t, err := parseInt(rawTimeout)
		if err != nil {
			return shellCommandRequest{}, errors.New("argument 'timeout_seconds' must be an integer")
		}
		if t < minimumShellTimeout {
			t = minimumShellTimeout
		}
		if t > maximumShellTimeout {
			t = maximumShellTimeout
		}
		timeoutSeconds = t
	}

	reason := ""
	if rawReason, ok := args["reason"]; ok {
		if s, ok := rawReason.(string); ok {
			reason = strings.TrimSpace(s)
		}
	}

	return shellCommandRequest{
		Command:          strings.TrimSpace(command),
		WorkingDirectory: wd,
		TimeoutSeconds:   timeoutSeconds,
		Reason:           reason,
	}, nil
}

func resolveWorkingDirectory(raw any) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if raw == nil {
		return cwd, nil
	}

	wd, ok := raw.(string)
	if !ok || strings.TrimSpace(wd) == "" {
		return cwd, nil
	}

	if filepath.IsAbs(wd) {
		return filepath.Clean(wd), nil
	}

	return filepath.Clean(filepath.Join(cwd, wd)), nil
}

func parseInt(v any) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int32:
		return int(n), nil
	case int64:
		return int(n), nil
	case float64:
		return int(n), nil
	case string:
		parsed, err := strconv.Atoi(n)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, errors.New("unsupported number type")
	}
}

type shellCommandResult struct {
	Request      shellCommandRequest
	Stdout       string
	Stderr       string
	ExitCode     int
	Duration     time.Duration
	TimedOut     bool
	ExecutionErr string
	StdoutCut    bool
	StderrCut    bool
}

func executeShellCommand(req shellCommandRequest) shellCommandResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.TimeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-lc", req.Command)
	cmd.Dir = req.WorkingDirectory

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
	execErr := ""

	if err != nil {
		execErr = err.Error()
		exitCode = -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		}
	}

	trimmedStdout, stdoutCut := truncateOutput(stdout.String(), maxShellOutputLength)
	trimmedStderr, stderrCut := truncateOutput(stderr.String(), maxShellOutputLength)

	return shellCommandResult{
		Request:      req,
		Stdout:       trimmedStdout,
		Stderr:       trimmedStderr,
		ExitCode:     exitCode,
		Duration:     duration,
		TimedOut:     timedOut,
		ExecutionErr: execErr,
		StdoutCut:    stdoutCut,
		StderrCut:    stderrCut,
	}
}

func truncateOutput(s string, max int) (string, bool) {
	runes := []rune(s)
	if len(runes) <= max {
		return s, false
	}
	return string(runes[:max]) + "\n...[truncated]", true
}

func (r shellCommandResult) toToolResponse() map[string]any {
	payload := map[string]any{
		"command":           r.Request.Command,
		"working_directory": r.Request.WorkingDirectory,
		"timeout_seconds":   r.Request.TimeoutSeconds,
		"exit_code":         r.ExitCode,
		"duration_ms":       r.Duration.Milliseconds(),
		"stdout":            r.Stdout,
		"stderr":            r.Stderr,
		"stdout_truncated":  r.StdoutCut,
		"stderr_truncated":  r.StderrCut,
		"timed_out":         r.TimedOut,
	}

	if r.ExecutionErr == "" && !r.TimedOut && r.ExitCode == 0 {
		return map[string]any{"output": payload}
	}

	errorPayload := map[string]any{"message": r.ExecutionErr}
	if r.TimedOut {
		errorPayload["message"] = "command timed out"
	}

	return map[string]any{
		"output": payload,
		"error":  errorPayload,
	}
}



// ============================================================================
// read_file tool implementation
// ============================================================================

type readFileResult struct {
	Request       readFileRequest
	Content       string
	TotalLines    int
	StartLine     int
	EndLine       int
	Truncated     bool
	ExecutionErr  string
}

func parseReadFileRequest(args map[string]any) (readFileRequest, error) {
	if args == nil {
		return readFileRequest{}, errors.New("function args missing")
	}

	pathValue, ok := args["path"]
	if !ok {
		return readFileRequest{}, errors.New("missing required argument: path")
	}

	path, ok := pathValue.(string)
	if !ok || strings.TrimSpace(path) == "" {
		return readFileRequest{}, errors.New("argument 'path' must be a non-empty string")
	}

	req := readFileRequest{
		Path:      strings.TrimSpace(path),
		StartLine: 0,
		EndLine:   0,
	}

	if rawStart, ok := args["start_line"]; ok {
		start, err := parseInt(rawStart)
		if err != nil {
			return readFileRequest{}, errors.New("argument 'start_line' must be an integer")
		}
		if start < 1 {
			return readFileRequest{}, errors.New("argument 'start_line' must be >= 1")
		}
		req.StartLine = start
	}

	if rawEnd, ok := args["end_line"]; ok {
		end, err := parseInt(rawEnd)
		if err != nil {
			return readFileRequest{}, errors.New("argument 'end_line' must be an integer")
		}
		if end < 1 {
			return readFileRequest{}, errors.New("argument 'end_line' must be >= 1")
		}
		req.EndLine = end
	}

	if req.StartLine > 0 && req.EndLine > 0 && req.StartLine > req.EndLine {
		return readFileRequest{}, errors.New("start_line cannot be greater than end_line")
	}

	return req, nil
}

func executeReadFile(req readFileRequest) readFileResult {
	resolvedPath := req.Path
	if !filepath.IsAbs(req.Path) {
		cwd, err := os.Getwd()
		if err != nil {
			return readFileResult{
				Request:      req,
				ExecutionErr: fmt.Sprintf("failed to get working directory: %v", err),
			}
		}
		resolvedPath = filepath.Join(cwd, req.Path)
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return readFileResult{
			Request:      req,
			ExecutionErr: fmt.Sprintf("failed to read file: %v", err),
		}
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	startLine := 1
	endLine := totalLines

	if req.StartLine > 0 {
		startLine = req.StartLine
	}
	if req.EndLine > 0 {
		endLine = req.EndLine
	}

	if startLine > totalLines {
		return readFileResult{
			Request:      req,
			ExecutionErr: fmt.Sprintf("start_line %d exceeds total lines %d", startLine, totalLines),
		}
	}

	if endLine > totalLines {
		endLine = totalLines
	}

	var builder strings.Builder
	for i := startLine - 1; i < endLine; i++ {
		builder.WriteString(fmt.Sprintf("%d: %s\n", i+1, lines[i]))
	}

	content := builder.String()
	truncated := false

	if len(content) > maxFileOutputLength {
		runes := []rune(content)
		content = string(runes[:maxFileOutputLength]) + "\n...[truncated]"
		truncated = true
	}

	return readFileResult{
		Request:    req,
		Content:    content,
		TotalLines: totalLines,
		StartLine:  startLine,
		EndLine:    endLine,
		Truncated:  truncated,
	}
}

func (r readFileResult) toToolResponse() map[string]any {
	if r.ExecutionErr != "" {
		return map[string]any{
			"error": map[string]any{
				"message": r.ExecutionErr,
			},
		}
	}

	header := ""
	if r.StartLine > 1 || r.EndLine < r.TotalLines {
		header = fmt.Sprintf("Lines %d-%d of %d:\n", r.StartLine, r.EndLine, r.TotalLines)
	} else {
		header = fmt.Sprintf("Total lines: %d\n", r.TotalLines)
	}

	return map[string]any{
		"output": map[string]any{
			"path":        r.Request.Path,
			"content":     header + r.Content,
			"total_lines": r.TotalLines,
			"start_line":  r.StartLine,
			"end_line":    r.EndLine,
			"truncated":   r.Truncated,
		},
	}
}

// ============================================================================
// write_file tool implementation
// ============================================================================

type writeFileResult struct {
	Request       writeFileRequest
	MatchCount    int
	ModifiedLines string
	ExecutionErr  string
	UserDenied    bool
}

func parseWriteFileRequest(args map[string]any) (writeFileRequest, error) {
	if args == nil {
		return writeFileRequest{}, errors.New("function args missing")
	}

	path, err := requiredStringArg(args, "path")
	if err != nil {
		return writeFileRequest{}, err
	}

	oldStr, err := requiredStringArg(args, "old_str")
	if err != nil {
		return writeFileRequest{}, err
	}

	newStrValue, ok := args["new_str"]
	if !ok {
		return writeFileRequest{}, errors.New("missing required argument: new_str")
	}

	newStr, ok := newStrValue.(string)
	if !ok {
		return writeFileRequest{}, errors.New("argument 'new_str' must be a string")
	}

	reason := ""
	if rawReason, ok := args["reason"]; ok {
		if s, ok := rawReason.(string); ok {
			reason = strings.TrimSpace(s)
		}
	}

	return writeFileRequest{
		Path:   path,
		OldStr: oldStr,
		NewStr: newStr,
		Reason: reason,
	}, nil
}

func executeWriteFile(req writeFileRequest) writeFileResult {
	resolvedPath := req.Path
	if !filepath.IsAbs(req.Path) {
		cwd, err := os.Getwd()
		if err != nil {
			return writeFileResult{
				Request:      req,
				ExecutionErr: fmt.Sprintf("failed to get working directory: %v", err),
			}
		}
		resolvedPath = filepath.Join(cwd, req.Path)
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return writeFileResult{
			Request:      req,
			ExecutionErr: fmt.Sprintf("failed to read file: %v", err),
		}
	}

	content := string(data)
	matchCount := strings.Count(content, req.OldStr)

	if matchCount == 0 {
		return writeFileResult{
			Request:      req,
			MatchCount:   0,
			ExecutionErr: "old_str not found in file",
		}
	}

	if matchCount > 1 {
		return writeFileResult{
			Request:      req,
			MatchCount:   matchCount,
			ExecutionErr: fmt.Sprintf("old_str appears %d times in file. Please provide a more specific old_str that matches exactly once", matchCount),
		}
	}

	newContent := strings.Replace(content, req.OldStr, req.NewStr, 1)

	err = os.WriteFile(resolvedPath, []byte(newContent), 0644)
	if err != nil {
		return writeFileResult{
			Request:      req,
			MatchCount:   matchCount,
			ExecutionErr: fmt.Sprintf("failed to write file: %v", err),
		}
	}

	oldLines := strings.Split(req.OldStr, "\n")
	startLineNum := strings.Count(content[:strings.Index(content, req.OldStr)], "\n") + 1
	endLineNum := startLineNum + len(oldLines) - 1

	modifiedLines := fmt.Sprintf("lines %d-%d", startLineNum, endLineNum)
	if startLineNum == endLineNum {
		modifiedLines = fmt.Sprintf("line %d", startLineNum)
	}

	return writeFileResult{
		Request:       req,
		MatchCount:    matchCount,
		ModifiedLines: modifiedLines,
	}
}

func (r writeFileResult) toToolResponse() map[string]any {
	if r.ExecutionErr != "" {
		payload := map[string]any{
			"path":        r.Request.Path,
			"match_count": r.MatchCount,
		}
		return map[string]any{
			"error":  map[string]any{"message": r.ExecutionErr},
			"output": payload,
		}
	}

	return map[string]any{
		"output": map[string]any{
			"path":           r.Request.Path,
			"modified_lines": r.ModifiedLines,
			"success":        true,
		},
	}
}

func generateDiff(oldStr, newStr string) string {
	var diff strings.Builder

	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	for _, line := range oldLines {
		diff.WriteString(fmt.Sprintf("- %s\n", line))
	}

	for _, line := range newLines {
		diff.WriteString(fmt.Sprintf("+ %s\n", line))
	}

	return diff.String()
}



// ============================================================================
// clipboard tool implementation
// ============================================================================

type clipboardResult struct {
	Request      clipboardRequest
	Content      string
	CharCount    int
	Truncated    bool
	ExecutionErr string
}

func parseClipboardRequest(args map[string]any) (clipboardRequest, error) {
	if args == nil {
		return clipboardRequest{}, errors.New("function args missing")
	}

	action, err := requiredStringArg(args, "action")
	if err != nil {
		return clipboardRequest{}, err
	}

	action = strings.ToLower(action)
	if action != "read" && action != "write" {
		return clipboardRequest{}, errors.New("action must be either 'read' or 'write'")
	}

	content := ""
	if action == "write" {
		contentValue, ok := args["content"]
		if !ok {
			return clipboardRequest{}, errors.New("content is required when action is 'write'")
		}
		contentStr, ok := contentValue.(string)
		if !ok {
			return clipboardRequest{}, errors.New("content must be a string")
		}
		content = contentStr
	}

	return clipboardRequest{
		Action:  action,
		Content: content,
	}, nil
}

func detectClipboardTool() error {
	// Check if we have a display server
	display := os.Getenv("DISPLAY")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	if display == "" && waylandDisplay == "" {
		return errors.New("no display server detected ($DISPLAY and $WAYLAND_DISPLAY are not set). Clipboard operations require a graphical environment")
	}

	// Check for wl-clipboard tools
	if _, err := exec.LookPath("wl-paste"); err != nil {
		return errors.New("wl-paste not found. Please install wl-clipboard (e.g., sudo dnf install wl-clipboard)")
	}
	if _, err := exec.LookPath("wl-copy"); err != nil {
		return errors.New("wl-copy not found. Please install wl-clipboard (e.g., sudo dnf install wl-clipboard)")
	}

	return nil
}

func executeClipboard(req clipboardRequest) clipboardResult {
	if err := detectClipboardTool(); err != nil {
		return clipboardResult{
			Request:      req,
			ExecutionErr: err.Error(),
		}
	}

	if req.Action == "read" {
		return executeClipboardRead(req)
	}
	return executeClipboardWrite(req)
}

func executeClipboardRead(req clipboardRequest) clipboardResult {
	cmd := exec.Command("wl-paste", "--no-newline")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		stderrStr := stderr.String()
		// wl-paste returns error when clipboard is empty, check stderr
		if strings.Contains(stderrStr, "Nothing is copied") || strings.Contains(stderrStr, "No selection") {
			return clipboardResult{
				Request:   req,
				Content:   "",
				CharCount: 0,
			}
		}
		if stderrStr != "" {
			return clipboardResult{
				Request:      req,
				ExecutionErr: fmt.Sprintf("failed to read clipboard: %v (%s)", err, stderrStr),
			}
		}
		return clipboardResult{
			Request:      req,
			ExecutionErr: fmt.Sprintf("failed to read clipboard: %v", err),
		}
	}

	content := stdout.String()
	if content == "" {
		return clipboardResult{
			Request:   req,
			Content:   "",
			CharCount: 0,
		}
	}

	truncated := false
	if len(content) > maxClipboardOutputLength {
		runes := []rune(content)
		content = string(runes[:maxClipboardOutputLength])
		truncated = true
	}

	return clipboardResult{
		Request:   req,
		Content:   content,
		CharCount: len([]rune(stdout.String())),
		Truncated: truncated,
	}
}

func executeClipboardWrite(req clipboardRequest) clipboardResult {
	clipboardDaemonMutex.Lock()
	defer clipboardDaemonMutex.Unlock()

	// Kill any existing clipboard daemon
	if clipboardDaemonCmd != nil && clipboardDaemonCmd.Process != nil {
		_ = clipboardDaemonCmd.Process.Kill()
		clipboardDaemonCmd = nil
	}

	// Start wl-copy as a background daemon
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(req.Content)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Start the process but don't wait for it
	err := cmd.Start()
	if err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return clipboardResult{
				Request:      req,
				ExecutionErr: fmt.Sprintf("failed to write clipboard: %v (%s)", err, stderrStr),
			}
		}
		return clipboardResult{
			Request:      req,
			ExecutionErr: fmt.Sprintf("failed to write clipboard: %v", err),
		}
	}

	// Store the command reference so we can kill it later
	clipboardDaemonCmd = cmd

	// Give it a moment to read stdin and set up the clipboard
	time.Sleep(50 * time.Millisecond)

	return clipboardResult{
		Request:   req,
		CharCount: len([]rune(req.Content)),
	}
}

func (r clipboardResult) toToolResponse() map[string]any {
	if r.ExecutionErr != "" {
		return map[string]any{
			"error": map[string]any{
				"message": r.ExecutionErr,
			},
		}
	}

	if r.Request.Action == "read" {
		if r.Content == "" {
			return map[string]any{
				"output": map[string]any{
					"action":  "read",
					"content": "",
					"message": "Clipboard is empty",
				},
			}
		}

		output := map[string]any{
			"action":     "read",
			"content":    r.Content,
			"char_count": r.CharCount,
		}

		if r.Truncated {
			output["truncated"] = true
			output["message"] = fmt.Sprintf("Content truncated at %d characters", maxClipboardOutputLength)
		}

		return map[string]any{"output": output}
	}

	// write action
	return map[string]any{
		"output": map[string]any{
			"action":     "write",
			"char_count": r.CharCount,
			"success":    true,
			"message":    fmt.Sprintf("Wrote %d characters to clipboard", r.CharCount),
		},
	}
}

