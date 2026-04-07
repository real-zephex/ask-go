package main

import (
	"fmt"
	"os"
)

func helpMenu() {
	fmt.Println(`
╔════════════════════════════════════════╗
║           ASK - CLI AI Assistant       ║
╚════════════════════════════════════════╝

USAGE:
  ask [flags] <prompt>
  ask --help

EXAMPLES:
  ask "what is a goroutine"
  ask "explain interfaces in go"
  ask --model exp "analyze this architecture deeply"
  ask --reason HIGH "design a scalable queue worker system"
  cat main.go | ask "explain this code"
  tail -n 50 app.log | ask --model cheap "summarize errors"

MODEL ALIASES:
  free         gemma-4-26b-a4b-it (default)
  cheap        gemini-3.1-flash-lite-preview
  exp          gemini-3-flash-preview

REASONING LEVELS:
  MIN          minimal reasoning (default)
  LOW          light reasoning
  MED          medium reasoning
  HIGH         high reasoning effort

FLAGS:
  --help        Show this help menu
  --version     Show current version
  --model       Model name or alias: free|cheap|exp
  --reason      Reasoning effort: HIGH|MED|LOW|MIN
  --clear       Clear local conversation history database
    `)
	os.Exit(0)
}
