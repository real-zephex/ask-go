package main

import (
	"bufio"
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
	"time"

	"google.golang.org/genai"
)

const (
	maxAgentToolRounds   = 20 
	defaultShellTimeout  = 30
	minimumShellTimeout  = 1
	maximumShellTimeout  = 180
	maxShellOutputLength = 8000
)

type shellCommandRequest struct {
	Command          string
	WorkingDirectory string
	TimeoutSeconds   int
	Reason           string
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
			response := handleAgentFunctionCall(call, autoApprove)
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

func handleAgentFunctionCall(call *genai.FunctionCall, autoApprove bool) map[string]any {
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

func askForCommandApproval() bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Approve command? [y/N]: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		v := strings.ToLower(strings.TrimSpace(line))
		switch v {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		}
	}
}
