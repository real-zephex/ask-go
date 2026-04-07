package main

import (
	"fmt"
	"log"

	"github.com/charmbracelet/glamour"
)

func render(text string) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		log.Fatal(err)
	}

	out, err := renderer.Render(text)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(out)
}
