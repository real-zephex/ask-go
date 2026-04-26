package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	promptStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	streamStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	finalStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	toolStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("204"))
	warnStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	memoryStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	memoryOKStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
)

func printREPLHeader(model string, reasoning string, stream bool, agent bool, yolo bool) {
	fmt.Println(headerStyle.Render("ask • interactive mode"))
	fmt.Println(subtleStyle.Render("commands: /help for slash commands"))
	fmt.Println(subtleStyle.Render(
		"model: " + model +
			" • reasoning: " + reasoning +
			" • stream: " + fmt.Sprintf("%t", stream) +
			" • agent: " + fmt.Sprintf("%t", agent) +
			" • yolo: " + fmt.Sprintf("%t", yolo),
	))
	fmt.Println()
}

func chatPrompt() string {
	return promptStyle.Render("ask ❯ ")
}

func printThinking() {
	fmt.Print(statusStyle.Render("thinking..."))
}

func clearThinking() {
	fmt.Print("\r" + strings.Repeat(" ", 24) + "\r")
}

func printStreamingLabel() {
	fmt.Println(streamStyle.Render("↳ streaming rendered markdown:"))
}

func printFinalRenderLabel() {
	fmt.Println(finalStyle.Render("↳ rendered markdown:"))
}

func printToolCall(req shellCommandRequest) {
	fmt.Println(toolStyle.Render("↳ tool: run_shell_command"))
	if req.Reason != "" {
		fmt.Println(subtleStyle.Render("reason: " + req.Reason))
	}
	fmt.Println(subtleStyle.Render("cwd: " + req.WorkingDirectory + " • timeout: " + fmt.Sprintf("%ds", req.TimeoutSeconds)))
	fmt.Println("$ " + req.Command)
}

func printToolDenied() {
	fmt.Println(warnStyle.Render("command denied by user"))
}

func printToolResult(result shellCommandResult) {
	status := "ok"
	if result.ExecutionErr != "" || result.ExitCode != 0 || result.TimedOut {
		status = "error"
	}
	fmt.Println(subtleStyle.Render(
		fmt.Sprintf("tool result: %s • exit=%d • duration=%dms", status, result.ExitCode, result.Duration.Milliseconds()),
	))
}

func printMemorySaved(stored int) {
	fmt.Println(memoryOKStyle.Render(fmt.Sprintf("🧠 memory: saved %d item(s)", stored)))
}

func printMemoryNoop() {
	fmt.Println(memoryStyle.Render("🧠 memory: no new items saved"))
}

func printMemoryWarning(err error) {
	fmt.Println(warnStyle.Render(fmt.Sprintf("🧠 memory warning: %v", err)))
}

func printMemoryWait(reason string, pending int64) {
	fmt.Println(memoryStyle.Render(fmt.Sprintf("%s. Please wait, finishing %d memory task(s)...", reason, pending)))
}

func printMemorySyncComplete() {
	fmt.Println(memoryOKStyle.Render("🧠 memory sync complete."))
}

type markdownStreamPreview struct {
	buffer            strings.Builder
	lastRenderAt      time.Time
	lastRenderedLines int
	minInterval       time.Duration
}

func newMarkdownStreamPreview() *markdownStreamPreview {
	return &markdownStreamPreview{
		minInterval: 120 * time.Millisecond,
	}
}

func (p *markdownStreamPreview) onChunk(chunk string) {
	if chunk == "" {
		return
	}

	p.buffer.WriteString(chunk)
	if time.Since(p.lastRenderAt) < p.minInterval && !strings.Contains(chunk, "\n") {
		return
	}
	p.renderCurrent()
}

func (p *markdownStreamPreview) onComplete(finalText string) {
	p.buffer.Reset()
	p.buffer.WriteString(finalText)
	p.renderCurrent()

	if !strings.HasSuffix(finalText, "\n") {
		fmt.Println()
	}
}

func (p *markdownStreamPreview) renderCurrent() {
	out := renderToString(p.buffer.String())
	if p.lastRenderedLines > 0 {
		fmt.Printf("\033[%dA", p.lastRenderedLines)
		fmt.Print("\033[J")
	}

	fmt.Print(out)
	p.lastRenderedLines = visualLineCount(out)
	p.lastRenderAt = time.Now()
}

func visualLineCount(s string) int {
	if s == "" {
		return 0
	}

	count := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		count++
	}
	if count == 0 {
		return 1
	}
	return count
}
