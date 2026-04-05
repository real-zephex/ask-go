package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
)

type Model string

const (
	free  Model = "gemma-4-26b-a4b-it"
	cheap Model = "gemini-3.1-flash-lite-preview"
	exp   Model = "gemini-3-flash-preview"
)

func resolveModels(m string) string {
	switch m {
	case "free":
		return string(free)
	case "cheap":
		return string(cheap)
	case "exp":
		return string(exp)
	default:
		return m
	}
}

var help = flag.Bool("help", false, "Show help menu")
var model = flag.String(
	"model",
	string(free),
	"the model name, e.g. gemma-4-26b-a4b-it")

func checkForEnv() (string, bool) {
	value, exists := os.LookupEnv("GEMINI_API_KEY")
	if !exists {
		log.Fatal("GEMINI API KEY not found in the PATH. Add the key to the path and restart the program")
	}
	return value, true
}

func main() {
	var stdinInput string
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		stdinInput = string(bytes)
	}

	flag.Parse()
	ctx := context.Background()

	if *help {
		helpMenu()
		os.Exit(0)
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		log.Fatal(err)
	}

	e, _ := checkForEnv()
	args := flag.Args()
	if len(args) == 0 {
		helpMenu()
		os.Exit(1)
	}
	query := strings.Join(args, " ")
	if stdinInput != "" && query != "" {
		query = query + "\n\n" + stdinInput
	} else if stdinInput != "" {
		query = stdinInput
	}
	if query == "" {
		helpMenu()
		os.Exit(1)
	}

	fmt.Print("thinking...")
	res := run(ctx, e, query, resolveModels(*model))
	fmt.Print("\r				\r")

	out, err := renderer.Render(res)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(out)
}
