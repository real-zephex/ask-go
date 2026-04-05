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
  ask <prompt>
  ask --help

EXAMPLES:
  ask "what is a goroutine"
  ask "explain interfaces in go"
  echo "main.go" | ask "explain this code"

FLAGS:
  --help        Show this help menu
    `)
	os.Exit(0)
}
